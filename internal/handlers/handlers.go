package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"Eressleep/go-musthave-diploma-tpl/internal/auth"
	"Eressleep/go-musthave-diploma-tpl/internal/middleware"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type Handlers struct {
	storage storage.Repository
	auth    *auth.Manager
	logger  *zap.Logger
}

func New(s storage.Repository, a *auth.Manager, logger *zap.Logger) *Handlers {
	return &Handlers{
		storage: s,
		auth:    a,
		logger:  logger,
	}
}

func (h *Handlers) Router() chi.Router {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	r.Post("/api/user/register", h.Register)
	r.Post("/api/user/login", h.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(h.auth))
		r.Post("/api/user/orders", h.UploadOrder)
		r.Get("/api/user/orders", h.ListOrders)
		r.Get("/api/user/balance", h.Balance)
		r.Post("/api/user/balance/withdraw", h.Withdraw)
		r.Get("/api/user/withdrawals", h.Withdrawals)
	})

	return r
}

func (h *Handlers) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.Error("encode response", zap.Error(err))
		}
	}
}

func (h *Handlers) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
