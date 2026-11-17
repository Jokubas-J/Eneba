package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned when no product has the requested slug. The HTTP layer
// turns it into a 404 response.
var ErrNotFound = errors.New("not found")

// Seller is the public part of a seller, shown on each offer row.
type Seller struct {
	Name        string  `json:"name"`
	Rating      float64 `json:"rating"` // 0-10, one decimal
	RatingCount int     `json:"ratingCount"`
	IsVerified  bool    `json:"isVerified"`
}

// Offer is one seller's price for the product, with its stock.
type Offer struct {
	ID                 int64  `json:"id"`
	Seller             Seller `json:"seller"`
	PriceCents         int    `json:"priceCents"`
	OriginalPriceCents int    `json:"originalPriceCents"`
	DiscountPercent    int    `json:"discountPercent"`
	// Stock is the number of keys that can be bought right now. 0 = sold out; the
	// offer is still listed, so a visitor sees that the seller exists.
	Stock int `json:"stock"`
	// IsCheapest marks the cheapest offer that is in stock: the "Best price" row and
	// the offer in the buy box. Exactly one offer has it, or none when all are sold out.
	IsCheapest bool `json:"isCheapest"`
}

// ProductVersion is the same game on another platform or for another region: one
// entry of the "Other versions" strip.
type ProductVersion struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Platform   Named  `json:"platform"`
	Region     Named  `json:"region"`
	PriceCents *int   `json:"priceCents"` // cheapest in stock; null when sold out
	OfferCount int    `json:"offerCount"`
}

// ProductDetail is everything the product page shows.
type ProductDetail struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	CoverURL    string  `json:"coverUrl"`
	Platform    Named   `json:"platform"`
	Region      Named   `json:"region"`
	Genres      []Named `json:"genres"`
	ReleaseDate *string `json:"releaseDate"` // YYYY-MM-DD; null when unknown
	Developer   string  `json:"developer"`
	Publisher   string  `json:"publisher"`
	SteamAppID  *int    `json:"steamAppId"`
	// Offers: in stock first, cheapest first; sold-out offers at the end.
	Offers        []Offer          `json:"offers"`
	OtherVersions []ProductVersion `json:"otherVersions"`
}

// Product loads one product with its offers and other versions.
//
// The four reads (product, genres, offers, versions) depend only on the product's id
// and its Steam app id, so after the first query the other three are sent together
// in one batch, inside one read-only transaction: the page sees one consistent
// snapshot of prices and stock.
func (r *Repository) Product(ctx context.Context, slug string) (ProductDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return ProductDetail{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var p ProductDetail
	var released *time.Time
	err = tx.QueryRow(ctx, `
		SELECT p.id, p.slug, p.name, p.description, p.product_type, p.cover_url,
		       pl.slug, pl.name, r.slug, r.name,
		       p.release_date, p.developer, p.publisher, p.steam_app_id
		FROM products p
		JOIN platforms pl ON pl.id = p.platform_id
		JOIN regions r ON r.id = p.region_id
		WHERE p.slug = $1`, slug).Scan(
		&p.ID, &p.Slug, &p.Name, &p.Description, &p.Type, &p.CoverURL,
		&p.Platform.Slug, &p.Platform.Name, &p.Region.Slug, &p.Region.Name,
		&released, &p.Developer, &p.Publisher, &p.SteamAppID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductDetail{}, ErrNotFound
	}
	if err != nil {
		return ProductDetail{}, fmt.Errorf("load product: %w", err)
	}
	if released != nil {
		date := released.Format("2006-01-02")
		p.ReleaseDate = &date
	}

	batch := &pgx.Batch{}
	batch.Queue(`
		SELECT g.slug, g.name
		FROM product_genres pg JOIN genres g ON g.id = pg.genre_id
		WHERE pg.product_id = $1
		ORDER BY g.name`, p.ID)
	// The stock of an offer is the number of its available keys (a correlated
	// subquery per offer, answered from the partial index on available keys).
	// The outer query exists so ORDER BY can use the computed stock column.
	batch.Queue(`
		SELECT id, seller_name, seller_rating, seller_rating_count, seller_verified,
		       price_cents, original_price_cents, stock
		FROM (
			SELECT o.id, s.name AS seller_name, s.rating::float8 AS seller_rating,
			       s.rating_count AS seller_rating_count, s.is_verified AS seller_verified,
			       o.price_cents, o.original_price_cents,
			       (SELECT count(*) FROM license_keys k
			        WHERE k.offer_id = o.id AND k.status = 'available')::int AS stock
			FROM offers o
			JOIN sellers s ON s.id = o.seller_id
			WHERE o.product_id = $1 AND o.is_active
		) offer
		ORDER BY stock = 0, price_cents, seller_rating DESC, id`, p.ID)
	batch.Queue(`
		SELECT slug, name, platform_slug, platform_name, region_slug, region_name, price_cents, offer_count
		FROM product_listing
		WHERE steam_app_id = $1 AND id <> $2
		ORDER BY platform_name, region_name, id`, p.SteamAppID, p.ID)

	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	rows, err := results.Query()
	if err != nil {
		return ProductDetail{}, fmt.Errorf("load genres: %w", err)
	}
	if p.Genres, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Named]); err != nil {
		return ProductDetail{}, fmt.Errorf("read genres: %w", err)
	}

	rows, err = results.Query()
	if err != nil {
		return ProductDetail{}, fmt.Errorf("load offers: %w", err)
	}
	p.Offers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Offer, error) {
		var o Offer
		err := row.Scan(&o.ID, &o.Seller.Name, &o.Seller.Rating, &o.Seller.RatingCount, &o.Seller.IsVerified,
			&o.PriceCents, &o.OriginalPriceCents, &o.Stock)
		return o, err
	})
	if err != nil {
		return ProductDetail{}, fmt.Errorf("read offers: %w", err)
	}

	rows, err = results.Query()
	if err != nil {
		return ProductDetail{}, fmt.Errorf("load other versions: %w", err)
	}
	p.OtherVersions, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProductVersion, error) {
		var v ProductVersion
		err := row.Scan(&v.Slug, &v.Name, &v.Platform.Slug, &v.Platform.Name, &v.Region.Slug, &v.Region.Name,
			&v.PriceCents, &v.OfferCount)
		return v, err
	})
	if err != nil {
		return ProductDetail{}, fmt.Errorf("read other versions: %w", err)
	}
	return p, nil
}

// Product returns the product page data with the derived offer fields filled in.
func (s *Service) Product(ctx context.Context, slug string) (ProductDetail, error) {
	p, err := s.repo.Product(ctx, slug)
	if err != nil {
		return ProductDetail{}, err
	}
	for i := range p.Offers {
		o := &p.Offers[i]
		o.DiscountPercent = DiscountPercent(o.PriceCents, o.OriginalPriceCents)
	}
	markCheapest(p.Offers)
	return p, nil
}

// markCheapest sets IsCheapest on the cheapest offer that has stock. It looks at the
// prices itself instead of trusting the order the query returned, because this flag
// decides which offer the buy box sells. Equal prices: the better-rated seller wins,
// then the lower id, so the choice is always the same.
func markCheapest(offers []Offer) {
	best := -1
	for i, o := range offers {
		if o.Stock == 0 {
			continue
		}
		if best == -1 || isBetterOffer(o, offers[best]) {
			best = i
		}
	}
	if best >= 0 {
		offers[best].IsCheapest = true
	}
}

func isBetterOffer(a, b Offer) bool {
	if a.PriceCents != b.PriceCents {
		return a.PriceCents < b.PriceCents
	}
	if a.Seller.Rating != b.Seller.Rating {
		return a.Seller.Rating > b.Seller.Rating
	}
	return a.ID < b.ID
}
