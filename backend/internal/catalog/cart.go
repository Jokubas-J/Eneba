package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The cart lives in the browser (localStorage) as a list of offer ids and
// quantities, so it works before signing in. It holds no names or prices: those are
// always looked up here, so what the cart page shows is today's price and stock,
// and nothing the browser sends can change a price.

const (
	// MaxCartLines and MaxLineQuantity bound a cart; real carts are far smaller,
	// and the limits keep one request from asking for thousands of rows.
	MaxCartLines    = 50
	MaxLineQuantity = 10
)

// CartLine is one line as the browser sends it.
type CartLine struct {
	OfferID  int64 `json:"offerId"`
	Quantity int   `json:"quantity"`
}

// Problems a cart line can have. The empty string means "can be bought as is".
const (
	CartProblemNone          = ""
	CartProblemNotFound      = "not_found"        // offer removed or deactivated
	CartProblemSoldOut       = "sold_out"         // no key in stock
	CartProblemNotEnoughKeys = "not_enough_stock" // fewer keys than the quantity
)

// CartItem is one validated line: the offer as it is now.
type CartItem struct {
	OfferID  int64 `json:"offerId"`
	Quantity int   `json:"quantity"` // as requested
	// The fields below are empty when Problem is not_found.
	Product            CartProduct `json:"product"`
	Seller             Seller      `json:"seller"`
	PriceCents         int         `json:"priceCents"`
	OriginalPriceCents int         `json:"originalPriceCents"`
	DiscountPercent    int         `json:"discountPercent"`
	Stock              int         `json:"stock"`
	Problem            string      `json:"problem"`
}

// CartProduct is what a cart line shows about the product.
type CartProduct struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CoverURL string `json:"coverUrl"`
	Platform Named  `json:"platform"`
	Region   Named  `json:"region"`
}

// ValidatedCart is the answer to POST /api/cart/validate.
type ValidatedCart struct {
	Items []CartItem `json:"items"`
	// TotalCents covers only the lines without a problem: what checkout would charge.
	TotalCents int `json:"totalCents"`
	// ItemCount is the number of keys in those lines.
	ItemCount int `json:"itemCount"`
}

// ValidateCart returns the current state of every requested line, in the order
// given. Invalid input (too many lines, a quantity out of range, the same offer
// twice) is an ErrInvalidFilter-style error: the browser's cart code never sends it.
func (s *Service) ValidateCart(ctx context.Context, lines []CartLine) (ValidatedCart, error) {
	if len(lines) > MaxCartLines {
		return ValidatedCart{}, fmtInvalid(fmt.Sprintf("a cart has at most %d lines", MaxCartLines))
	}
	ids := make([]int64, 0, len(lines))
	seen := map[int64]bool{}
	for _, line := range lines {
		if line.Quantity < 1 || line.Quantity > MaxLineQuantity {
			return ValidatedCart{}, fmtInvalid(fmt.Sprintf("quantity must be between 1 and %d", MaxLineQuantity))
		}
		if seen[line.OfferID] {
			return ValidatedCart{}, fmtInvalid("an offer appears twice in the cart")
		}
		seen[line.OfferID] = true
		ids = append(ids, line.OfferID)
	}

	found, err := s.repo.CartOffers(ctx, ids)
	if err != nil {
		return ValidatedCart{}, err
	}

	cart := ValidatedCart{Items: make([]CartItem, 0, len(lines))}
	for _, line := range lines {
		item, ok := found[line.OfferID]
		if !ok {
			cart.Items = append(cart.Items, CartItem{OfferID: line.OfferID, Quantity: line.Quantity, Problem: CartProblemNotFound})
			continue
		}
		item.Quantity = line.Quantity
		item.DiscountPercent = DiscountPercent(item.PriceCents, item.OriginalPriceCents)
		switch {
		case item.Stock == 0:
			item.Problem = CartProblemSoldOut
		case item.Stock < line.Quantity:
			item.Problem = CartProblemNotEnoughKeys
		default:
			// Integer cents throughout: no rounding anywhere in a total.
			cart.TotalCents += item.PriceCents * line.Quantity
			cart.ItemCount += line.Quantity
		}
		cart.Items = append(cart.Items, item)
	}
	return cart, nil
}

// CartOffers loads the active offers with the given ids, with product, seller and
// stock, keyed by offer id. Inactive or unknown ids are simply absent.
func (r *Repository) CartOffers(ctx context.Context, ids []int64) (map[int64]CartItem, error) {
	items := map[int64]CartItem{}
	if len(ids) == 0 {
		return items, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, p.slug, p.name, p.cover_url, pl.slug, pl.name, r.slug, r.name,
		       s.name, s.rating::float8, s.rating_count, s.is_verified,
		       o.price_cents, o.original_price_cents,
		       (SELECT count(*) FROM license_keys k WHERE k.offer_id = o.id AND k.status = 'available')::int
		FROM offers o
		JOIN products p ON p.id = o.product_id
		JOIN platforms pl ON pl.id = p.platform_id
		JOIN regions r ON r.id = p.region_id
		JOIN sellers s ON s.id = o.seller_id
		WHERE o.id = ANY($1) AND o.is_active`, ids)
	if err != nil {
		return nil, fmt.Errorf("load cart offers: %w", err)
	}
	found, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CartItem, error) {
		var c CartItem
		err := row.Scan(&c.OfferID, &c.Product.Slug, &c.Product.Name, &c.Product.CoverURL,
			&c.Product.Platform.Slug, &c.Product.Platform.Name, &c.Product.Region.Slug, &c.Product.Region.Name,
			&c.Seller.Name, &c.Seller.Rating, &c.Seller.RatingCount, &c.Seller.IsVerified,
			&c.PriceCents, &c.OriginalPriceCents, &c.Stock)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("read cart offers: %w", err)
	}
	for _, item := range found {
		items[item.OfferID] = item
	}
	return items, nil
}
