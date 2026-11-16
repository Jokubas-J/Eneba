package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Jokubas-J/Eneba/backend/internal/config"
)

// testDatabaseURL returns the connection string of the database that tests may use,
// or skips the test when none is configured.
//
// Tests must never touch the development or production data, so they read ONLY
// TEST_DATABASE_URL and refuse to run if it points at the same database as
// DATABASE_URL (a copy-paste mistake in .env would otherwise go unnoticed).
func testDatabaseURL(t *testing.T) string {
	t.Helper()

	// "go test" runs with the package folder as working directory, so backend/.env
	// is two levels up from backend/internal/database.
	if err := config.LoadDotEnv("../../.env"); err != nil {
		t.Fatalf("read .env: %v", err)
	}

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database test")
	}
	if url == os.Getenv("DATABASE_URL") {
		t.Fatal("TEST_DATABASE_URL equals DATABASE_URL; tests must use their own database")
	}
	return url
}

func TestConnectAndMigrate(t *testing.T) {
	url := testDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer pool.Close()

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	// Running it again must be a no-op: the server calls Migrate at every start.
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	// similarity() exists only if the pg_trgm migration really ran.
	var score float32
	if err := pool.QueryRow(ctx, `SELECT similarity('witcher', 'witchr')`).Scan(&score); err != nil {
		t.Fatalf("pg_trgm is not available: %v", err)
	}
	if score <= 0 || score >= 1 {
		t.Errorf("similarity('witcher', 'witchr') = %v, want a value between 0 and 1", score)
	}
}

func TestSchema(t *testing.T) {
	url := testDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	t.Run("all tables exist", func(t *testing.T) {
		tables := []string{
			"platforms", "regions", "genres", "products", "product_genres",
			"sellers", "offers", "license_keys",
			"users", "sessions", "orders", "order_items", "payment_events",
			"banners", "home_blocks",
		}
		for _, table := range tables {
			var exists bool
			// to_regclass returns NULL when no table with that name exists.
			err := pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists)
			if err != nil {
				t.Fatalf("look up table %s: %v", table, err)
			}
			if !exists {
				t.Errorf("table %s does not exist", table)
			}
		}
	})

	t.Run("a key cannot be sold without belonging to an order item", func(t *testing.T) {
		// Everything happens in a transaction that is rolled back, so the test
		// leaves no rows behind whatever its outcome.
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)

		var offerID int64
		err = tx.QueryRow(ctx, `
			WITH p AS (INSERT INTO platforms (slug, name) VALUES ('test-platform', 'Test') RETURNING id),
			     r AS (INSERT INTO regions (slug, name) VALUES ('test-region', 'Test') RETURNING id),
			     pr AS (
			         INSERT INTO products (slug, name, platform_id, region_id)
			         SELECT 'test-product', 'Test product', p.id, r.id FROM p, r
			         RETURNING id
			     ),
			     s AS (INSERT INTO sellers (name, rating) VALUES ('Test seller', 9.5) RETURNING id)
			INSERT INTO offers (product_id, seller_id, price_cents, original_price_cents)
			SELECT pr.id, s.id, 1000, 2000 FROM pr, s
			RETURNING id`).Scan(&offerID)
		if err != nil {
			t.Fatalf("insert test offer: %v", err)
		}

		// A normal available key is accepted...
		if _, err := tx.Exec(ctx,
			`INSERT INTO license_keys (offer_id, key_value) VALUES ($1, 'TEST-KEY-AVAILABLE')`, offerID); err != nil {
			t.Fatalf("insert available key: %v", err)
		}

		// ...but a sold key that points at no order item must be rejected by the
		// CHECK constraint. This must be the last statement: PostgreSQL aborts the
		// whole transaction after an error.
		_, err = tx.Exec(ctx,
			`INSERT INTO license_keys (offer_id, key_value, status) VALUES ($1, 'TEST-KEY-SOLD', 'sold')`, offerID)

		// errors.As walks the wrapped error chain looking for the PostgreSQL error,
		// like "catch (PostgresException e)" in C#. 23514 = check_violation.
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("insert of a sold key without order item: error = %v, want check violation (23514)", err)
		}
	})
}

func TestConnectFailsOnUnreachableDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Port 1 on localhost: nothing listens there, so the connection is refused at once.
	pool, err := Connect(ctx, "postgres://user:pass@127.0.0.1:1/nothing?sslmode=disable")
	if err == nil {
		pool.Close()
		t.Fatal("Connect() to an unreachable database returned no error")
	}
}
