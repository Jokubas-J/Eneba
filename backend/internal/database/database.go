// Package database opens the PostgreSQL connection pool and applies schema migrations.
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// The migration files are compiled INTO the binary by this directive, so the server
// that Render runs is a single file that carries its own schema history. Without it
// the .sql files would have to be copied into the Docker image and found at runtime.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Connect opens a pool of connections and checks that the database answers.
//
// A pool (rather than one connection) is needed because the HTTP server handles
// requests concurrently and a single PostgreSQL connection can run only one query
// at a time. The pool is safe to share between goroutines.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// pgxpool.New connects lazily, so a wrong password would only surface on the
	// first query. Ping forces a real connection now. The timeout is generous
	// because a Neon database that was idle needs a few seconds to wake up.
	pingCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies every migration that has not been applied to this database yet.
// goose records applied versions in its own table (goose_db_version), so running
// this at every start-up is safe: when the schema is current it does nothing.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// goose speaks Go's generic database/sql interface, while the application uses
	// pgx directly. This adapter exposes the same pool through database/sql, so no
	// second set of connections is opened. Closing it does not close the pool.
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	// The embedded file system contains "migrations/0001_x.sql"; goose expects the
	// files at the root, so fs.Sub re-roots it at the migrations folder.
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("locate embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, files)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, result := range results {
		log.Printf("migration applied: %s", result)
	}
	return nil
}
