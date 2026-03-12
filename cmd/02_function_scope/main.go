// Approach 2 — Function Scope Injection
//
// Handler factories close over singleton dependencies (DB, Logger, etc.).
// UserRepository and UserService are value types created fresh on every request.
// The handler function itself is the scope boundary.
//
// Lifecycle:
//   Singleton  → DB, Logger, Validator, EmailService (closed over by factories)
//   Scoped     → RequestID (middleware)
//   Transient  → UserRepository, UserService (new allocation per request)
//
// Run: go run ./cmd/02_function_scope/
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/di_in_go/internal/adapters/repository"
	"github.com/example/di_in_go/internal/adapters/rest"
	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

// makeCreateHandler returns an http.HandlerFunc that closes over singletons.
// On each request it creates a new UserRepository and UserService (Transient).
// No struct fields hold services — the function IS the composition root.
func makeCreateHandler(
	db *infradb.MemoryDB,
	log application.Logger,
	emailSvc application.EmailService,
	val application.Validator,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ── Transient: created fresh for this request, discarded after ──
		repo := repository.NewMemory(db, log)
		svc := application.NewUserService(repo, emailSvc, val, log)
		// ────────────────────────────────────────────────────────────────

		var req domain.CreateUserRequest
		if err := rest.ReadJSON(r, &req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		u, err := svc.Create(r.Context(), req)
		if err != nil {
			log.Error("create user", "err", err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		rest.WriteJSON(w, http.StatusCreated, u)
	}
}

func makeGetHandler(db *infradb.MemoryDB, log application.Logger,
	emailSvc application.EmailService, val application.Validator,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ── Transient ──
		repo := repository.NewMemory(db, log)
		svc := application.NewUserService(repo, emailSvc, val, log)
		// ──────────────

		var id int64
		fmt.Sscanf(r.PathValue("id"), "%d", &id)
		u, err := svc.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		rest.WriteJSON(w, http.StatusOK, u)
	}
}

func main() {
	// ── Singletons ──
	log := logger.New()
	db := infradb.New()
	val := validator.New()
	emailSvc := email.NewStub(log)

	mux := http.NewServeMux()
	// Handler factories receive singletons; they close over them.
	mux.HandleFunc("POST /users", makeCreateHandler(db, log, emailSvc, val))
	mux.HandleFunc("GET /users/{id}", makeGetHandler(db, log, emailSvc, val))

	srv := &http.Server{
		Addr:    ":8081",
		Handler: rest.Chain(mux, rest.RequestIDMiddleware),
	}

	go func() {
		log.Info("02_function_scope listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	srv.Shutdown(context.Background())
}
