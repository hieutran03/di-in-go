// Package rest provides HTTP adapters: handler, middleware, and scoped-service helpers.
package rest

import (
	"encoding/json"
	"net/http"
)

// WriteJSON serialises v as JSON with the given status code.
// Exported so cmd packages that build functional handlers can use it directly.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ReadJSON decodes the request body into dst.
// Exported for the same reason as WriteJSON.
func ReadJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// writeJSON / readJSON are package-level aliases used by handler.go and scope.go.
func writeJSON(w http.ResponseWriter, status int, v any) { WriteJSON(w, status, v) }
func readJSON(r *http.Request, dst any) error            { return ReadJSON(r, dst) }
