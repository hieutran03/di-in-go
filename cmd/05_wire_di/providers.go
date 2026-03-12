// Approach 5 — Compile-time DI (Google Wire style)
//
// Provider functions are pure constructors.  The Wire CLI analyses their
// signatures, resolves the dependency graph, and emits wire_gen.go with InitApp().
// No reflection at runtime — the generated code is ordinary Go.
//
// Usage with the real Wire CLI:
//
//	go install github.com/google/wire/cmd/wire@latest
//	wire ./cmd/05_wire_di/     # regenerates wire_gen.go
//
// Without the CLI, just run: go run ./cmd/05_wire_di/
// (wire_gen.go is already committed and calls the providers manually.)
package main

import (
	"github.com/example/di_in_go/internal/adapters/repository"
	"github.com/example/di_in_go/internal/adapters/rest"
	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
	infradb "github.com/example/di_in_go/internal/infrastructure/db"
	"github.com/example/di_in_go/internal/infrastructure/email"
	"github.com/example/di_in_go/internal/infrastructure/logger"
	"github.com/example/di_in_go/internal/infrastructure/validator"
)

// ── Provider functions ────────────────────────────────────────────────────────
//
// Each provider is a pure constructor whose parameter types are matched by Wire
// to the return types of other providers.  You write these; Wire writes InitApp().

func ProvideLogger() application.Logger { return logger.New() }

func ProvideDB() *infradb.MemoryDB { return infradb.New() }

func ProvideValidator() application.Validator { return validator.New() }

func ProvideEmailService(log application.Logger) application.EmailService {
	return email.NewStub(log)
}

// ProvideUserRepository provides the persistence port.
// Wire sees: needs (*infradb.MemoryDB, application.Logger), produces domain.UserRepository.
func ProvideUserRepository(db *infradb.MemoryDB, log application.Logger) domain.UserRepository {
	return repository.NewMemory(db, log)
}

// ProvideUserService provides the application use case.
// Wire sees: needs (domain.UserRepository, application.EmailService, ...), produces application.UserService.
func ProvideUserService(
	repo domain.UserRepository,
	emailSvc application.EmailService,
	val application.Validator,
	log application.Logger,
) application.UserService {
	return application.NewUserService(repo, emailSvc, val, log)
}

// ProvideUserHandler provides the HTTP adapter.
func ProvideUserHandler(svc application.UserService, log application.Logger) *rest.UserHandler {
	return rest.NewUserHandler(svc, log)
}
