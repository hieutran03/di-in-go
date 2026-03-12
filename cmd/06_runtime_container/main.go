// Approach 6 — Runtime DI Container (Uber Dig / Fx style)
//
// The container.Container (in infrastructure/container) mirrors go.uber.org/dig.
// Providers are registered in any order; the graph is resolved lazily on Invoke.
// OnStart / OnStop hooks model Fx's lifecycle API.
//
// Lifecycle:
//   Singleton  → all registered types (container caches by type after first resolve)
//   Scoped     → model via a child container per request (not shown here)
//   Transient  → provider returns func() T; caller invokes the factory
//
// Run: go run ./cmd/06_runtime_container/
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
	"github.com/example/di_in_go/internal/infrastructure/container"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

// Server bundles the HTTP server so the container can manage its lifecycle.
type Server struct{ srv *http.Server }

func newServer(h *rest.UserHandler) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("GET /users/{id}", h.Get)
	return &Server{srv: &http.Server{
		Addr:    ":8085",
		Handler: rest.Chain(mux, rest.RequestIDMiddleware),
	}}
}

// provideUserService is a two-step provider: first build the repo, then the service.
// In real Dig you would register NewMemory and NewUserService separately and let
// the container thread the repo through automatically.
func provideUserService(db *infradb.MemoryDB, emailSvc application.EmailService,
	val application.Validator, log application.Logger,
) application.UserService {
	repo := repository.NewMemory(db, log)
	return application.NewUserService(repo, emailSvc, val, log)
}

func main() {
	c := container.New()

	// ── Registration — order independent ──────────────────────────────────────
	// The container resolves these lazily; you can register in any order.
	c.Provide(logger.New)       // () → application.Logger
	c.Provide(infradb.New)      // () → *infradb.MemoryDB
	c.Provide(validator.New)    // () → application.Validator
	c.Provide(email.NewStub)    // (application.Logger) → application.EmailService
	c.Provide(provideUserService) // (...) → application.UserService
	c.Provide(rest.NewUserHandler) // (application.UserService, application.Logger) → *rest.UserHandler
	c.Provide(newServer)        // (*rest.UserHandler) → *Server

	// ── Lifecycle hooks ───────────────────────────────────────────────────────
	c.OnStart(func(_ context.Context) error {
		return c.Invoke(func(s *Server, log application.Logger) error {
			go func() {
				log.Info("06_runtime_container listening", "addr", s.srv.Addr)
				if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error("server error", "err", err)
				}
			}()
			return nil
		})
	})

	c.OnStop(func(ctx context.Context) error {
		return c.Invoke(func(s *Server) error {
			return s.srv.Shutdown(ctx)
		})
	})

	// ── Start ──────────────────────────────────────────────────────────────────
	// Resolution happens here: the container walks the graph, calling each
	// provider at most once and caching results (Singleton behaviour).
	if err := c.Start(context.Background()); err != nil {
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()
	c.Stop(ctx)
}
