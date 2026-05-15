package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"Eressleep/go-musthave-diploma-tpl/internal/luhn"
	"Eressleep/go-musthave-diploma-tpl/internal/middleware"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"go.uber.org/zap"
)

type balanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type withdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func (h *Handlers) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	b, err := h.storage.Balance(r.Context(), userID)
	if err != nil {
		h.logger.Error("get balance", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.respondJSON(w, http.StatusOK, balanceResponse{
		Current:   b.Current,
		Withdrawn: b.Withdrawn,
	})
}

func (h *Handlers) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	if req.Sum <= 0 {
		h.respondError(w, http.StatusUnprocessableEntity, "sum must be positive")
		return
	}

	if !luhn.Valid(req.Order) {
		h.respondError(w, http.StatusUnprocessableEntity, "invalid order number format")
		return
	}

	err := h.storage.Withdraw(r.Context(), userID, req.Order, req.Sum)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, storage.ErrInsufficientFunds):
		h.respondError(w, http.StatusPaymentRequired, "insufficient funds")
	default:
		h.logger.Error("withdraw", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
	}
}

func (h *Handlers) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, err := h.storage.WithdrawalsByUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("list withdrawals", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	out := make([]withdrawalResponse, 0, len(items))
	for _, it := range items {
		out = append(out, withdrawalResponse{
			Order:       it.OrderNumber,
			Sum:         it.Sum,
			ProcessedAt: it.ProcessedAt.Format(time.RFC3339),
		})
	}

	h.respondJSON(w, http.StatusOK, out)
}
