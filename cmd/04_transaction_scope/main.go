// Approach 4 — Transaction Scope Pattern (Unit of Work)
//
// application.WithTransaction begins a Tx, injects it into context, calls the
// callback, then commits or rolls back.  Inside the callback, UnitOfWork.Users()
// returns a txUserRepo that reads the Tx from context — no Tx parameter drilling.
//
// This is the Go equivalent of @Transactional (Spring) / TransactionScope (.NET).
//
// Lifecycle:
//
//	Singleton  → DB, Logger, Validator, EmailService, UnitOfWork (factory)
//	Scoped     → domain.Tx (alive for the WithTransaction callback)
//	             txUserRepo (produced by UoW per callback)
//	Transient  → none
//
// See README.md for the Create flow and when to use.
//
// Run: go run ./cmd/04_transaction_scope/
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

// txUserService wraps the application.UserService to add explicit transaction scope.
// It delegates to application.WithTransaction and then calls the base service.
type txUserService struct {
	base  application.UserService
	uow   application.UnitOfWork
	db    domain.TxStarter
	email application.EmailService
	val   application.Validator
	log   application.Logger
}

func newTxUserService(
	uow application.UnitOfWork,
	db domain.TxStarter,
	emailSvc application.EmailService,
	val application.Validator,
	log application.Logger,
) application.UserService {
	return &txUserService{
		uow:   uow,
		db:    db,
		email: emailSvc,
		val:   val,
		log:   log,
	}
}

func (s *txUserService) Create(ctx context.Context, req domain.CreateUserRequest) (domain.User, error) {
	if err := s.val.ValidateCreateUser(req.Name, req.Email); err != nil {
		return domain.User{}, fmt.Errorf("validation: %w", err)
	}

	var created domain.User

	// ── Transaction Scope boundary ──────────────────────────────────────────
	// WithTransaction injects the Tx into txCtx.
	// Any repo obtained via s.uow.Users(txCtx) will use that Tx automatically.
	err := application.WithTransaction(ctx, s.db, func(txCtx context.Context) error {
		repo := s.uow.Users(txCtx) // Scoped: repo shares the active Tx
		u, err := repo.Create(txCtx, domain.User{Name: req.Name, Email: req.Email})
		if err != nil {
			return err
		}
		// Additional repos here would share the same Tx:
		// profileRepo := s.uow.Profiles(txCtx)
		created = u
		return nil
	})
	// ────────────────────────────────────────────────────────────────────────

	if err != nil {
		return domain.User{}, err
	}
	_ = s.email.SendWelcome(ctx, created.Email, created.Name) // post-commit
	return created, nil
}

func (s *txUserService) GetByID(ctx context.Context, id int64) (domain.User, error) {
	// Reads don't require a transaction; use the read-only repo.
	repo := s.uow.Users(ctx)
	return repo.GetByID(ctx, id)
}

func main() {
	// ── Singletons ──
	log := logger.New()
	db := infradb.New()
	val := validator.New()
	emailSvc := email.NewStub(log)
	uow := repository.NewTxMemoryUoW(db, log)

	svc := newTxUserService(uow, db, emailSvc, val, log)
	h := rest.NewUserHandler(svc, log)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.Create)
	mux.HandleFunc("GET /users/{id}", h.Get)

	srv := &http.Server{
		Addr:    ":8083",
		Handler: rest.Chain(mux, rest.RequestIDMiddleware),
	}

	go func() {
		log.Info("04_transaction_scope listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	srv.Shutdown(context.Background())
}
