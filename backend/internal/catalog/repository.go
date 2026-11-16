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

	tx, err := r.beginRead(ctx)
	if err != nil {
		return nil, 0, err
	}
	// Nothing is written, so there is nothing to commit; rolling back just ends it.
	defer tx.Rollback(ctx)

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

// beginRead starts a read-only transaction prepared for the search conditions.
//
// A transaction is needed because the similarity threshold is set with
// transaction-local scope; "read only" makes PostgreSQL refuse any write by mistake.
func (r *Repository) beginRead(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	// The "<%" operator used by the search compares against this setting. The third
	// argument "true" makes it local to this transaction (like SET LOCAL), so the
	// pooled connection goes back with its default.
	_, err = tx.Exec(ctx, `SELECT set_config('pg_trgm.word_similarity_threshold', $1, true)`, similarityThreshold)
	if err != nil {
		tx.Rollback(ctx)
		return nil, fmt.Errorf("set similarity threshold: %w", err)
	}
	return tx, nil
}

// Facets returns, for every filter group, each option with the number of products
// the store would show if that option were ticked.
//
// Each group is counted with all the OTHER filters applied but not its own. With
// "Steam" ticked, the platform group must still say how many GOG products there
// are, otherwise every unticked platform would show 0 and adding a second platform
// would look pointless. So each group needs its own WHERE clause, and therefore its
// own query; the four are sent together in one batch (one network round trip).
func (r *Repository) Facets(ctx context.Context, f ListFilter) (Facets, error) {
	// For each group: the filter with that group's own selection removed.
	// "f" is passed by value, so each assignment below changes only a copy.
	withoutPlatforms, withoutRegions, withoutGenres, withoutTypes := f, f, f, f
	withoutPlatforms.Platforms = nil
	withoutRegions.Regions = nil
	withoutGenres.Genres = nil
	withoutTypes.Types = nil

	batch := &pgx.Batch{}

	// The option list comes from the lookup table and is LEFT JOINed to the matching
	// products, so an option with no match is still returned, with count 0.
	// count(l.id) counts only joined rows: for an option without products the join
	// yields one row of NULLs, which count(column) ignores (count(*) would say 1).
	q := buildQuery(withoutPlatforms)
	batch.Queue(`
		SELECT o.slug, o.name, count(l.id)::int
		FROM platforms o
		LEFT JOIN (SELECT id, platform_slug FROM product_listing `+q.where()+`) l ON l.platform_slug = o.slug
		GROUP BY o.id
		ORDER BY count(l.id) DESC, o.name`, q.args...)

	q = buildQuery(withoutRegions)
	batch.Queue(`
		SELECT o.slug, o.name, count(l.id)::int
		FROM regions o
		LEFT JOIN (SELECT id, region_slug FROM product_listing `+q.where()+`) l ON l.region_slug = o.slug
		GROUP BY o.id
		ORDER BY count(l.id) DESC, o.name`, q.args...)

	// A product is counted once under each of its genres, so the genre counts can
	// add up to more than the number of products. That is intended.
	q = buildQuery(withoutGenres)
	batch.Queue(`
		SELECT o.slug, o.name, count(l.product_id)::int
		FROM genres o
		LEFT JOIN (
			SELECT link.product_id, link.genre_id
			FROM product_listing
			JOIN product_genres link ON link.product_id = product_listing.id
			`+q.where()+`
		) l ON l.genre_id = o.id
		GROUP BY o.id
		ORDER BY count(l.product_id) DESC, o.name`, q.args...)

	// Product types have no lookup table; the options are the types that exist.
	q = buildQuery(withoutTypes)
	batch.Queue(`
		SELECT o.product_type, o.product_type, count(l.id)::int
		FROM (SELECT DISTINCT product_type FROM products) o
		LEFT JOIN (SELECT id, product_type FROM product_listing `+q.where()+`) l ON l.product_type = o.product_type
		GROUP BY o.product_type
		ORDER BY count(l.id) DESC, o.product_type`, q.args...)

	tx, err := r.beginRead(ctx)
	if err != nil {
		return Facets{}, err
	}
	defer tx.Rollback(ctx)

	results := tx.SendBatch(ctx, batch)
	// The connection is busy until the batch results are closed.
	defer results.Close()

	// Results come back in the order the queries were queued.
	var facets Facets
	for _, group := range []*[]FacetOption{&facets.Platforms, &facets.Regions, &facets.Genres, &facets.Types} {
		rows, err := results.Query()
		if err != nil {
			return Facets{}, fmt.Errorf("facet query: %w", err)
		}
		options := []FacetOption{}
		for rows.Next() {
			var option FacetOption
			if err := rows.Scan(&option.Slug, &option.Name, &option.Count); err != nil {
				rows.Close()
				return Facets{}, fmt.Errorf("read facet row: %w", err)
			}
			options = append(options, option)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Facets{}, fmt.Errorf("read facet rows: %w", err)
		}
		// "group" is a pointer to one of the four fields of facets.
		*group = options
	}
	return facets, nil
}
