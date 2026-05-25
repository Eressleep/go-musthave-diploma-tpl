package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"Eressleep/go-musthave-diploma-tpl/internal/luhn"
	"Eressleep/go-musthave-diploma-tpl/internal/middleware"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"go.uber.org/zap"
)

type orderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

func (h *Handlers) UploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()

	number := strings.TrimSpace(string(body))
	if number == "" {
		h.respondError(w, http.StatusBadRequest, "empty order number")
		return
	}

	if !isDigitsOnly(number) {
		h.respondError(w, http.StatusUnprocessableEntity, "invalid order number format")
		return
	}

	if !luhn.Valid(number) {
		h.respondError(w, http.StatusUnprocessableEntity, "invalid order number format")
		return
	}

	err = h.storage.CreateOrder(r.Context(), number, userID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, storage.ErrOrderAlreadyUploaded):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, storage.ErrOrderOwnedByOther):
		h.respondError(w, http.StatusConflict, "order already uploaded by another user")
	default:
		h.logger.Error("create order", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
	}
}

func (h *Handlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orders, err := h.storage.OrdersByUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("list orders", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	out := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderResponse{
			Number:     o.Number,
			Status:     string(o.Status),
			Accrual:    o.Accrual,
			UploadedAt: o.UploadedAt.Format(time.RFC3339),
		})
	}

	h.respondJSON(w, http.StatusOK, out)
}

func isDigitsOnly(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
