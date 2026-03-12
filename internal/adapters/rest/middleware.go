package rest

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
)

// Chain composes middleware right-to-left so the first element is the outermost layer.
//
//	Chain(mux, A, B, C) → A(B(C(mux)))
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// ── RequestIDMiddleware ───────────────────────────────────────────────────────
//
// Generates a unique correlation ID and injects it into context.
// Lifecycle: creates one string value per request → Scoped.

func RequestIDMiddleware(next http.Handler) http.Handler {
	var counter atomic.Int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("req-%d", counter.Add(1))
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

// ── AuthMiddleware ────────────────────────────────────────────────────────────
//
// Validates the caller's identity and injects domain.AuthUser into context.
// Lifecycle: creates one AuthUser value per request → Scoped.

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Production: parse and validate Authorization header / session cookie.
		// Here a fixed user is simulated for demonstration.
		u := domain.AuthUser{ID: 1, Roles: []string{"user"}}
		next.ServeHTTP(w, r.WithContext(domain.WithAuthUser(r.Context(), u)))
	})
}

// ── TxMiddleware ──────────────────────────────────────────────────────────────
//
// Begins a DB transaction, injects it into context, then commits or rolls back
// after the handler returns.  The transaction lifetime equals one HTTP request.
// Lifecycle: opens one domain.Tx per request → Scoped.

func TxMiddleware(starter domain.TxStarter, log application.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tx, err := starter.BeginTx()
			if err != nil {
				log.Error("begin tx", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			rw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r.WithContext(domain.WithTx(r.Context(), tx)))

			if rw.statusCode() >= 500 {
				tx.Rollback()
			} else {
				if commitErr := tx.Commit(); commitErr != nil {
					log.Error("tx commit", "err", commitErr)
				}
			}
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the written status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if sw.status == 0 {
		sw.status = http.StatusOK
	}
	return sw.ResponseWriter.Write(b)
}

func (sw *statusWriter) statusCode() int {
	if sw.status == 0 {
		return http.StatusOK
	}
	return sw.status
}
