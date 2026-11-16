package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

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

// errorBody is the one error shape of the whole API:
//
//	{"error": {"code": "invalid_request", "message": "unknown sort"}}
//
// "code" is for programs (the frontend switches on it), "message" is for people.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeInternalError is for failures that are our fault (database down, bug).
// The real error goes to the server log only: its text can contain table names or
// connection details that must not be shown to visitors.
func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong. Please try again.")
}
