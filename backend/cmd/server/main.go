// Command server is the entry point of the backend: it wires the pieces together
// and starts the HTTP server. All real logic lives in the internal/ packages.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/httpapi"
)

func main() {
	// Render tells the process which port to listen on through PORT; locally it is
	// unset and we fall back to 8080 (the port the Vite dev proxy points at).
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// http.ListenAndServe(addr, handler) would be shorter, but it has no timeouts at
	// all: a client that connects and sends nothing would hold a connection forever.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on http://localhost:%s", port)
	// ListenAndServe blocks for the lifetime of the server and only returns on failure
	// (for example, the port is already in use), so any return here is fatal.
	log.Fatal(srv.ListenAndServe())
}
