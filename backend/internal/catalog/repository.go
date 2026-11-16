package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the only place in this package that talks to the database.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// orderBy maps every Sort to a complete, constant ORDER BY clause.
//
// Each one ends with "id" so the order is total: without a unique last key, rows
// with equal values could swap places between two queries and a product could show
// up on page 1 and again on page 2.
// NULLS LAST keeps sold-out products (no price) and unknown dates at the end in
// both directions; PostgreSQL would otherwise put NULLs first for DESC.
var orderBy = map[Sort]string{
	SortPopular:   "popularity DESC, id",
	SortPriceAsc:  "price_cents ASC NULLS LAST, id",
	SortPriceDesc: "price_cents DESC NULLS LAST, id",
	SortNewest:    "release_date DESC NULLS LAST, id",
	SortOldest:    "release_date ASC NULLS LAST, id",
	SortName:      "lower(name), id",
}

// buildWhere turns a filter into a WHERE clause with numbered placeholders and the
// matching argument list.
//
// Values from the request are NEVER written into the SQL text; they travel
// separately as arguments ($1, $2, ...), which is what prevents SQL injection.
// Only the fixed condition strings below are concatenated.
func buildWhere(f ListFilter) (string, []any) {
	var conditions []string
	var args []any

	// add appends one condition; "$?" in it is replaced by the next placeholder number.
	add := func(condition string, arg any) {
		args = append(args, arg)
		conditions = append(conditions, strings.Replace(condition, "$?", fmt.Sprintf("$%d", len(args)), 1))
	}

	// "= ANY($1)" with a list argument is the parameterised form of IN (...):
	// one placeholder however many boxes are ticked.
	if len(f.Platforms) > 0 {
		add("platform_slug = ANY($?)", f.Platforms)
	}
	if len(f.Regions) > 0 {
		add("region_slug = ANY($?)", f.Regions)
	}
	if len(f.Types) > 0 {
		add("product_type = ANY($?)", f.Types)
	}
	if len(f.Genres) > 0 {
		// A product has many genres, so this asks "does at least one of its genres
		// match?". EXISTS (rather than a JOIN) cannot duplicate the product row
		// when two of its genres are both selected.
		add(`EXISTS (
			SELECT 1 FROM product_genres pg
			JOIN genres g ON g.id = pg.genre_id
			WHERE pg.product_id = product_listing.id AND g.slug = ANY($?))`, f.Genres)
	}
	// A price bound also removes sold-out products: NULL >= 500 is not true.
	if f.PriceMin != nil {
		add("price_cents >= $?", *f.PriceMin)
	}
	if f.PriceMax != nil {
		add("price_cents <= $?", *f.PriceMax)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// List returns one page of products matching the filter and the total number of
// matches across all pages.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]ProductCard, int, error) {
	where, args := buildWhere(f)

	// Two queries: the count needs no sorting or paging, and unlike a
	// "count(*) OVER ()" column it still works when the requested page is empty.
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM product_listing `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	order, ok := orderBy[f.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("unknown sort %q", f.Sort)
	}

	// LIMIT and OFFSET are arguments too; they take the next two placeholder numbers.
	query := fmt.Sprintf(`
		SELECT id, slug, name, product_type, cover_url,
		       platform_slug, platform_name, region_slug, region_name,
		       price_cents, original_price_cents, offer_count
		FROM product_listing
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, where, order, len(args)+1, len(args)+2)
	args = append(args, PageSize, (f.Page-1)*PageSize)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	// An empty, non-nil slice: it becomes [] in JSON, where a nil slice would be null.
	items := []ProductCard{}
	for rows.Next() {
		var p ProductCard
		// Scanning a NULL into a *int leaves the pointer nil.
		err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Type, &p.CoverURL,
			&p.Platform.Slug, &p.Platform.Name, &p.Region.Slug, &p.Region.Name,
			&p.PriceCents, &p.OriginalPriceCents, &p.OfferCount)
		if err != nil {
			return nil, 0, fmt.Errorf("read product row: %w", err)
		}
		items = append(items, p)
	}
	// rows.Next() also returns false when the connection breaks mid-way; Err tells
	// the two cases apart.
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read product rows: %w", err)
	}
	return items, total, nil
}
