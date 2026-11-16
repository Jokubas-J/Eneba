package httpapi

import "net/http"

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
