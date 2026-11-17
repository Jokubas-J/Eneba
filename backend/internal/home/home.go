// Package home serves the homepage: the list of blocks (GET /api/home) and the data
// of each block, one endpoint per block (GET /api/home/banners, …/best-sellers,
// …/deals, …/new-releases, …/categories).
//
// Each block is independent: its own query, its own endpoint, its own failure. The
// browser loads them separately, so a slow or broken block costs only itself. They
// live in one package (and one server) because they are a few queries each; making
// them separate deployable services would add network hops and deployments without
// changing what the visitor sees. Design: docs/homepage.md.
package home

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

// CarouselSize is how many product cards a carousel block returns.
const CarouselSize = 12

// BestSellerWindowDays is how far back sales count for "Best sellers".
const BestSellerWindowDays = 30

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Block is one entry of the homepage layout (a row of home_blocks).
type Block struct {
	ID         int64   `json:"id"`
	Type       string  `json:"type"`  // which frontend component draws it
	Title      *string `json:"title"` // null for blocks without a heading (banners)
	Source     string  `json:"source"`
	ShowAllURL *string `json:"showAllUrl"`
}

// Blocks returns the active homepage blocks in display order.
func (s *Service) Blocks(ctx context.Context) ([]Block, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, title, source, show_all_url
		FROM home_blocks
		WHERE is_active
		ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load home blocks: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Block])
}

type Banner struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	ImageURL string `json:"imageUrl"` // wide art, for the large tile
	// TileImageURL is a portrait image for a narrow tile, where the wide art would
	// show only a sliver; null = use ImageURL there too.
	TileImageURL *string `json:"tileImageUrl"`
	LinkURL      string  `json:"linkUrl"`
}

// Banners returns the active banners; the first one is the large tile.
func (s *Service) Banners(ctx context.Context) ([]Banner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, subtitle, image_url, tile_image_url, link_url
		FROM banners
		WHERE is_active
		ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load banners: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Banner])
}

// BestSellers ranks in-stock products by keys sold in paid orders over the last
// BestSellerWindowDays days. Products without sales in that window follow, ordered
// by popularity (Steam's recommendation count), so the row is full from day one and a
// real purchase moves a game up. No sales are invented.
func (s *Service) BestSellers(ctx context.Context) ([]catalog.ProductCard, error) {
	return s.cards(ctx, `
		WITH sales AS (
			SELECT o.product_id, sum(i.quantity) AS units
			FROM order_items i
			JOIN orders ord ON ord.id = i.order_id
			JOIN offers o ON o.id = i.offer_id
			WHERE ord.status = 'paid'
			  AND ord.paid_at > now() - make_interval(days => $1)
			GROUP BY o.product_id
		)
		SELECT `+cardColumns+`
		FROM product_listing l
		LEFT JOIN sales s ON s.product_id = l.id
		WHERE l.offer_count > 0
		ORDER BY COALESCE(s.units, 0) DESC, l.popularity DESC, l.id
		LIMIT $2`, BestSellerWindowDays, CarouselSize)
}

// Deals returns the in-stock products whose cheapest offer is furthest below its
// reference price (largest percentage first).
func (s *Service) Deals(ctx context.Context) ([]catalog.ProductCard, error) {
	return s.cards(ctx, `
		SELECT `+cardColumns+`
		FROM product_listing l
		WHERE l.offer_count > 0 AND l.original_price_cents > l.price_cents
		ORDER BY (l.original_price_cents - l.price_cents)::numeric / l.original_price_cents DESC,
		         l.popularity DESC, l.id
		LIMIT $1`, CarouselSize)
}

// NewReleases returns the most recently released in-stock products. Future release
// dates are left out: those are pre-orders, not new releases.
func (s *Service) NewReleases(ctx context.Context) ([]catalog.ProductCard, error) {
	return s.cards(ctx, `
		SELECT `+cardColumns+`
		FROM product_listing l
		WHERE l.offer_count > 0 AND l.release_date <= current_date
		ORDER BY l.release_date DESC, l.popularity DESC, l.id
		LIMIT $1`, CarouselSize)
}

// Category is a genre tile with the number of products that can be bought now.
type Category struct {
	Slug  string  `json:"slug"`
	Name  string  `json:"name"`
	Count int     `json:"count"`
	Icon  *string `json:"icon"` // icon key from taxonomy_icons; null = the frontend's default
}

// Categories returns the genres that have something in stock, largest first.
func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.slug, g.name, count(*)::int, ti.icon
		FROM genres g
		JOIN product_genres pg ON pg.genre_id = g.id
		JOIN product_listing l ON l.id = pg.product_id
		LEFT JOIN taxonomy_icons ti ON ti.kind = 'genre' AND ti.slug = g.slug
		WHERE l.offer_count > 0
		GROUP BY g.id, ti.icon
		ORDER BY count(*) DESC, g.name`)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Category])
}

// cardColumns are the product_listing columns a product card needs, in the order
// cards() scans them. The carousels read the same view as the store, so a card's
// "From €X" price is the same everywhere.
const cardColumns = `l.id, l.slug, l.name, l.product_type, l.cover_url,
	l.platform_slug, l.platform_name, l.region_slug, l.region_name,
	l.price_cents, l.original_price_cents, l.offer_count`

func (s *Service) cards(ctx context.Context, sql string, args ...any) ([]catalog.ProductCard, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("load product cards: %w", err)
	}
	cards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (catalog.ProductCard, error) {
		var p catalog.ProductCard
		err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Type, &p.CoverURL,
			&p.Platform.Slug, &p.Platform.Name, &p.Region.Slug, &p.Region.Name,
			&p.PriceCents, &p.OriginalPriceCents, &p.OfferCount)
		if err == nil && p.PriceCents != nil && p.OriginalPriceCents != nil {
			discount := catalog.DiscountPercent(*p.PriceCents, *p.OriginalPriceCents)
			p.DiscountPercent = &discount
		}
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("read product cards: %w", err)
	}
	return cards, nil
}
