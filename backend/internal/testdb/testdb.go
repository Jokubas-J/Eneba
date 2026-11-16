// Package testdb gives integration tests a connection to the TEST database.
//
// It is a normal (non _test) package so that tests in several packages can share it,
// but it is imported only from test files and never ends up in the server.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Jokubas-J/Eneba/backend/internal/config"
	"github.com/Jokubas-J/Eneba/backend/internal/database"
)

// URL returns the connection string of the database that tests may use, or skips
// the calling test when none is configured.
//
// Tests must never touch the development or production data, so this reads ONLY
// TEST_DATABASE_URL and fails if it points at the same database as DATABASE_URL
// (a copy-paste mistake in .env would otherwise go unnoticed).
func URL(t testing.TB) string {
	t.Helper()

	// "go test" runs with the package folder as working directory. Every package
	// that uses this helper lives in backend/internal/<name>, so backend/.env is
	// two levels up.
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

// Open connects to the test database, brings its schema up to date and closes the
// pool automatically when the test ends.
func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := URL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return pool
}
