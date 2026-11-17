// Command server is the entry point of the backend: it wires the pieces together
// and starts the HTTP server. All real logic lives in the internal/ packages.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/auth"
	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/config"
	"github.com/Jokubas-J/Eneba/backend/internal/database"
	"github.com/Jokubas-J/Eneba/backend/internal/httpapi"
)

func main() {
	// main only reports the error and sets the exit code. The work is in run() so
	// that its deferred clean-up (closing the pool) actually executes: log.Fatal
	// exits the process immediately and skips deferred calls of the same function.
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Local development convenience; on Render there is no .env file and the
	// variables come from the service settings. Run the server from backend/.
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Migrations run before the server accepts requests, so handlers can rely on
	// the schema being current. A failed migration stops the start-up: serving
	// with a half-migrated schema would be worse than not serving.
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	// Dependencies are wired by hand, innermost first: repository (SQL) -> service
	// (rules) -> router (HTTP). No dependency-injection container: the whole object
	// graph of the program is readable in these lines.
	products := catalog.NewService(catalog.NewRepository(pool))
	accounts, err := auth.NewService(pool)
	if err != nil {
		return err
	}

	// http.ListenAndServe(addr, handler) would be shorter, but it has no timeouts at
	// all: a client that connects and sends nothing would hold a connection forever.
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Catalog:       products,
			Accounts:      accounts,
			AllowedOrigin: cfg.AppBaseURL,
			SecureCookies: cfg.AppEnv == "production",
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on port %s (%s)", cfg.Port, cfg.AppEnv)
	// ListenAndServe blocks for the lifetime of the server and only returns on
	// failure (for example, the port is already in use).
	return srv.ListenAndServe()
}
