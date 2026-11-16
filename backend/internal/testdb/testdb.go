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
// pool automatically when the test ends. The calling test has the database to
// itself until it finishes (see lock). Call it once per test function.
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

	lock(t, pool)

	// A fresh deadline: waiting for the lock may have used up the first one.
	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelMigrate()
	if err := database.Migrate(migrateCtx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return pool
}

// lockKey is an arbitrary number that identifies "the test database is in use".
const lockKey = 20261116

// lock makes tests that share the one test database take turns.
//
// "go test ./..." runs the tests of different packages at the same time, in separate
// processes, so a Go mutex cannot coordinate them. A PostgreSQL advisory lock can:
// it is held by a database session, and a second session asking for the same key
// waits until the first releases it. Without this, one package could empty the
// tables while a test of another package is reading them.
func lock(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()

	// Waiting for the tests of another package to finish can take a while.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// The lock belongs to one session, so one connection is taken out of the pool
	// and kept for the whole test.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection for the test lock: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockKey); err != nil {
		conn.Release()
		t.Fatalf("take the test database lock: %v", err)
	}

	// Cleanups run last-registered-first, so this runs before the pool is closed.
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey); err != nil {
			t.Logf("release the test database lock: %v", err)
		}
		conn.Release()
	})
}
