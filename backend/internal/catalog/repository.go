package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the only place in this package that talks to the database.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// orderBy maps every Sort to a complete, constant ORDER BY clause. (SortRelevance is
// not here: its clause contains the score expression, which depends on the query.)
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

// query collects the pieces of one SQL statement while it is being built.
type query struct {
	conditions []string
	args       []any
}

// arg registers a value as a query argument and returns its placeholder: the first
// call returns "$1", the second "$2", and so on.
//
// Values from the request are NEVER written into the SQL text; they travel
// separately as arguments, which is what prevents SQL injection. Only the fixed
// condition strings in this file are concatenated.
func (q *query) arg(value any) string {
	q.args = append(q.args, value)
	return fmt.Sprintf("$%d", len(q.args))
}

func (q *query) where() string {
	if len(q.conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(q.conditions, " AND ")
}

// buildQuery turns a filter into conditions and arguments.
func buildQuery(f ListFilter) *query {
	q := &query{}
	add := func(condition string) { q.conditions = append(q.conditions, condition) }

	if words := NormalizeQuery(f.Text); len(words) > 0 {
		add(searchCondition(words, q.arg))
	}

	// "= ANY($1)" with a list argument is the parameterised form of IN (...):
	// one placeholder however many boxes are ticked.
	if len(f.Platforms) > 0 {
		add("platform_slug = ANY(" + q.arg(f.Platforms) + ")")
	}
	if len(f.Regions) > 0 {
		add("region_slug = ANY(" + q.arg(f.Regions) + ")")
	}
	if len(f.Types) > 0 {
		add("product_type = ANY(" + q.arg(f.Types) + ")")
	}
	if len(f.Genres) > 0 {
		// A product has many genres, so this asks "does at least one of its genres
		// match?". EXISTS (rather than a JOIN) cannot duplicate the product row
		// when two of its genres are both selected.
		add(`EXISTS (
			SELECT 1 FROM product_genres pg
			JOIN genres g ON g.id = pg.genre_id
			WHERE pg.product_id = product_listing.id AND g.slug = ANY(` + q.arg(f.Genres) + `))`)
	}
	// A price bound also removes sold-out products: NULL >= 500 is not true.
	if f.PriceMin != nil {
		add("price_cents >= " + q.arg(*f.PriceMin))
	}
	if f.PriceMax != nil {
		add("price_cents <= " + q.arg(*f.PriceMax))
	}
	return q
}

// orderClause returns the ORDER BY for a sort. Best-match order needs a search
// text; without one there is nothing to rank by, so it falls back to popularity.
func orderClause(sort Sort, words []string, arg func(any) string) (string, error) {
	if sort == SortRelevance {
		if len(words) == 0 {
			return orderBy[SortPopular], nil
		}
		// Equal scores are common (every edition of one game matches "witcher"
		// equally well), so popularity decides among them, then id.
		return searchScore(words, arg) + " DESC, popularity DESC, id", nil
	}
	order, ok := orderBy[sort]
	if !ok {
		return "", fmt.Errorf("unknown sort %q", sort)
	}
	return order, nil
}

// List returns up to limit products matching the filter, starting at offset, and
// the total number of matches.
func (r *Repository) List(ctx context.Context, f ListFilter, limit, offset int) ([]ProductCard, int, error) {
	q := buildQuery(f)

	// One read-only transaction for both queries. It is needed for SET LOCAL below,
	// and "read only" lets PostgreSQL refuse any write by mistake.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, fmt.Errorf("begin: %w", err)
	}
	// Nothing is written, so there is nothing to commit; rolling back just ends it.
	defer tx.Rollback(ctx)

	// The "<%" operator used by the search compares against this setting. The third
	// argument "true" makes it local to this transaction (like SET LOCAL), so the
	// pooled connection goes back with its default.
	if _, err := tx.Exec(ctx, `SELECT set_config('pg_trgm.word_similarity_threshold', $1, true)`, similarityThreshold); err != nil {
		return nil, 0, fmt.Errorf("set similarity threshold: %w", err)
	}

	// Two queries: the count needs no sorting or paging, and unlike a
	// "count(*) OVER ()" column it still works when the requested page is empty.
	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM product_listing `+q.where(), q.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	// The count above was run with the WHERE arguments only. The ORDER BY of a search
	// and LIMIT/OFFSET add more arguments after them, with the next placeholder
	// numbers. (An argument that a statement does not use is an error in PostgreSQL,
	// which is why they are registered only now.)
	where := q.where()
	order, err := orderClause(f.Sort, NormalizeQuery(f.Text), q.arg)
	if err != nil {
		return nil, 0, err
	}
	sql := fmt.Sprintf(`
		SELECT id, slug, name, product_type, cover_url,
		       platform_slug, platform_name, region_slug, region_name,
		       price_cents, original_price_cents, offer_count
		FROM product_listing
		%s
		ORDER BY %s
		LIMIT %s OFFSET %s`, where, order, q.arg(limit), q.arg(offset))

	rows, err := tx.Query(ctx, sql, q.args...)
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
