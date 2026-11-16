package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// healthResponse is the JSON body of GET /api/healthz.
type healthResponse struct {
	Status string `json:"status"`
}

// handleHealth answers "is the process up?". It deliberately does not touch the
// database: the uptime pinger calls it every 10 minutes to keep the free Render
// instance awake, and that must not also keep the Neon database from going idle.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// writeJSON sends v as a JSON response with the given status code.
// Headers must be set before WriteHeader: once the status line is sent they are frozen.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status is already on the wire, so the error cannot be reported to the
		// client any more; logging is all that is left.
		log.Printf("writeJSON: %v", err)
	}
}
