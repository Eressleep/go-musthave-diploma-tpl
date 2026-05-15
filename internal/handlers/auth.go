package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"Eressleep/go-musthave-diploma-tpl/internal/auth"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"go.uber.org/zap"
)

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	creds, err := decodeCredentials(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	hash, err := auth.HashPassword(creds.Password)
	if err != nil {
		h.logger.Error("hash password", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.storage.CreateUser(r.Context(), creds.Login, hash)
	if err != nil {
		if errors.Is(err, storage.ErrLoginTaken) {
			h.respondError(w, http.StatusConflict, "login already taken")
			return
		}
		h.logger.Error("create user", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.issueToken(w, user.ID)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	creds, err := decodeCredentials(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	user, err := h.storage.UserByLogin(r.Context(), creds.Login)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			h.respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.logger.Error("get user", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, creds.Password) {
		h.respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.issueToken(w, user.ID)
}

func (h *Handlers) issueToken(w http.ResponseWriter, userID int64) {
	token, err := h.auth.Issue(userID)
	if err != nil {
		h.logger.Error("issue token", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func decodeCredentials(r *http.Request) (*credentials, error) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if c.Login == "" || c.Password == "" {
		return nil, errors.New("login and password are required")
	}

	return &c, nil
}
