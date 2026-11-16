package database

import (
	"context"
	"os"
	"testing"
	"time"

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
