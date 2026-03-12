package rest

import (
	"fmt"
	"net/http"

	"github.com/example/di_in_go/internal/application"
	"github.com/example/di_in_go/internal/domain"
)

// UserHandler is the HTTP adapter for application.UserService.
//
// Clean Architecture rule satisfied:
//   - Depends on application.UserService (interface) — not the concrete struct.
//   - Depends on domain types for request/response shapes.
//   - Zero knowledge of infrastructure (db, email, etc.).
//
// Lifecycle: Singleton — stateless struct; all mutable state lives in the service.
type UserHandler struct {
	svc application.UserService
	log application.Logger
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(svc application.UserService, log application.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateUserRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	u, err := h.svc.Create(r.Context(), req)
	if err != nil {
		h.log.Error("create user", "err", err, "request_id", RequestIDFromContext(r.Context()))
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	var id int64
	fmt.Sscanf(r.PathValue("id"), "%d", &id)
	u, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
