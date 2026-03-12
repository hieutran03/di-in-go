// Approach 1 — Manual Constructor Injection
//
// The simplest and most explicit wiring strategy.
// Every dependency is passed via constructor; the caller (main) controls order.
//
// Lifecycle:
//   Singleton  → all services and repositories (created once, shared)
//   Scoped     → RequestID (generated per request by RequestIDMiddleware)
//   Transient  → none
//
// Run: go run ./cmd/01_manual/
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/di_in_go/internal/adapters/repository"
	"github.com/example/di_in_go/internal/adapters/rest"
	"github.com/example/di_in_go/internal/application"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

func main() {
	// ── Singletons — created once, injected by pointer/interface everywhere ──
	log := logger.New()
	db := infradb.New()
	val := validator.New()
	emailSvc := email.NewStub(log)

	// Adapters depend on infrastructure through interfaces declared in domain/application.
	repo := repository.NewMemory(db, log)
	svc := application.NewUserService(repo, emailSvc, val, log)
	h := rest.NewUserHandler(svc, log)

	// ── Routes ──
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("GET /users/{id}", h.Get)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: rest.Chain(mux, rest.RequestIDMiddleware),
	}

	// ── Graceful shutdown ──
	go func() {
		log.Info("01_manual listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")
	srv.Shutdown(context.Background())
}
