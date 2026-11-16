package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Counts reports how many rows Load inserted.
type Counts struct {
	Products, Offers, Keys, Sellers, Genres int
}

// ErrNotEmpty is returned when the catalog already has products and reset is false.
var ErrNotEmpty = errors.New("the database already contains products; run again with -reset to replace them (this also deletes all orders)")

// Load writes the dataset to the database in ONE transaction: either the whole
// catalog is there afterwards, or (on any error) nothing has changed.
//
// It refuses to touch a database that already has products unless reset is true,
// because replacing offers and keys necessarily deletes the orders that refer to
// them. Accounts (users, sessions) are never touched.
func Load(ctx context.Context, pool *pgxpool.Pool, dataset Dataset, reset bool) (Counts, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Counts{}, err
	}
	// Rollback after a successful Commit does nothing, so deferring it covers every
	// early return below without repeating it.
	defer tx.Rollback(ctx)

	var existing int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&existing); err != nil {
		return Counts{}, err
	}
	if existing > 0 && !reset {
		return Counts{}, ErrNotEmpty
	}
	if reset {
		// RESTART IDENTITY makes ids start from 1 again, so a re-seed gives the same
		// ids (and therefore the same URLs and test expectations) as the first one.
		_, err := tx.Exec(ctx, `
			TRUNCATE license_keys, order_items, orders, payment_events,
			         offers, sellers, product_genres, products, genres, regions, platforms
			RESTART IDENTITY`)
		if err != nil {
			return Counts{}, fmt.Errorf("empty tables: %w", err)
		}
	}

	platformIDs, err := insertNamed(ctx, tx, "platforms", dataset.Platforms)
	if err != nil {
		return Counts{}, err
	}
	regionIDs, err := insertNamed(ctx, tx, "regions", dataset.Regions)
	if err != nil {
		return Counts{}, err
	}
	genreIDs, err := insertNamed(ctx, tx, "genres", dataset.Genres)
	if err != nil {
		return Counts{}, err
	}

	// A Batch queues many statements and sends them in a single network round trip.
	// The database is in another country, so 500 separate INSERTs would spend
	// almost all their time waiting on the network; batching makes it one wait.
	sellerIDs := map[string]int64{}
	batch := &pgx.Batch{}
	for _, s := range dataset.Sellers {
		batch.Queue(`INSERT INTO sellers (name, rating, rating_count, is_verified) VALUES ($1, $2, $3, $4) RETURNING id`,
			s.Name, s.Rating, s.RatingCount, s.IsVerified)
	}
	ids, err := sendBatch(ctx, tx, batch)
	if err != nil {
		return Counts{}, fmt.Errorf("insert sellers: %w", err)
	}
	for i, s := range dataset.Sellers {
		sellerIDs[s.Name] = ids[i]
	}

	batch = &pgx.Batch{}
	for _, p := range dataset.Products {
		// NULLIF turns the empty string into NULL for an unknown release date.
		batch.Queue(`
			INSERT INTO products (slug, name, description, product_type, platform_id, region_id,
			                      cover_url, release_date, developer, publisher, popularity, steam_app_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::date, $9, $10, $11, $12)
			RETURNING id`,
			p.Slug, p.Name, p.Description, p.Type, platformIDs[p.PlatformSlug], regionIDs[p.RegionSlug],
			p.CoverURL, p.ReleaseDate, p.Developer, p.Publisher, p.Popularity, p.SteamAppID)
	}
	productIDs, err := sendBatch(ctx, tx, batch)
	if err != nil {
		return Counts{}, fmt.Errorf("insert products: %w", err)
	}

	// Offers need the product ids, and keys need the offer ids, so each level is
	// inserted after the one above it has returned its ids.
	batch = &pgx.Batch{}
	var genreRows [][]any
	for i, p := range dataset.Products {
		for _, slug := range p.GenreSlugs {
			genreRows = append(genreRows, []any{productIDs[i], genreIDs[slug]})
		}
		for _, o := range p.Offers {
			batch.Queue(`INSERT INTO offers (product_id, seller_id, price_cents, original_price_cents) VALUES ($1, $2, $3, $4) RETURNING id`,
				productIDs[i], sellerIDs[o.SellerName], o.PriceCents, o.OriginalPriceCents)
		}
	}
	offerIDs, err := sendBatch(ctx, tx, batch)
	if err != nil {
		return Counts{}, fmt.Errorf("insert offers: %w", err)
	}

	// offerIDs is in the same order in which the offers were queued, so walking the
	// products and offers again in that order pairs each offer with its id.
	var keyRows [][]any
	next := 0
	for _, p := range dataset.Products {
		for _, o := range p.Offers {
			for _, key := range o.Keys {
				keyRows = append(keyRows, []any{offerIDs[next], key})
			}
			next++
		}
	}

	// CopyFrom uses PostgreSQL's COPY protocol, the fastest way to insert many rows
	// when no generated values have to come back (thousands of keys here).
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"product_genres"},
		[]string{"product_id", "genre_id"}, pgx.CopyFromRows(genreRows)); err != nil {
		return Counts{}, fmt.Errorf("insert product genres: %w", err)
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"license_keys"},
		[]string{"offer_id", "key_value"}, pgx.CopyFromRows(keyRows)); err != nil {
		return Counts{}, fmt.Errorf("insert license keys: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Counts{}, err
	}
	return Counts{
		Products: len(productIDs),
		Offers:   len(offerIDs),
		Keys:     len(keyRows),
		Sellers:  len(sellerIDs),
		Genres:   len(genreIDs),
	}, nil
}

// insertNamed fills one of the lookup tables and returns slug -> generated id.
// The table name is concatenated into the SQL, which is safe only because every
// caller passes a constant; it is never user input.
func insertNamed(ctx context.Context, tx pgx.Tx, table string, rows []Named) (map[string]int64, error) {
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`INSERT INTO `+table+` (slug, name) VALUES ($1, $2) RETURNING id`, row.Slug, row.Name)
	}
	ids, err := sendBatch(ctx, tx, batch)
	if err != nil {
		return nil, fmt.Errorf("insert %s: %w", table, err)
	}
	bySlug := make(map[string]int64, len(rows))
	for i, row := range rows {
		bySlug[row.Slug] = ids[i]
	}
	return bySlug, nil
}

// sendBatch runs a batch in which every statement ends with "RETURNING id" and
// returns the ids in the order the statements were queued.
func sendBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) ([]int64, error) {
	if batch.Len() == 0 {
		return nil, nil
	}
	results := tx.SendBatch(ctx, batch)
	// The transaction's connection is busy until the batch results are closed.
	defer results.Close()

	ids := make([]int64, batch.Len())
	for i := range ids {
		if err := results.QueryRow().Scan(&ids[i]); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
