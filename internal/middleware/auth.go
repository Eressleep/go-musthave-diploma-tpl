package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var (
	ErrNoAuthHeader = errors.New("authorization header is missing or malformed")
)

var userIDKey = ctxKey{}

type TokenParser interface {
	Parse(tokenString string) (int64, error)
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDKey).(int64)
	return v, ok && v > 0
}

func AuthMiddleware(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractToken(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userID, err := parser.Parse(token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", ErrNoAuthHeader
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", ErrNoAuthHeader
	}

	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if token == "" {
		return "", ErrNoAuthHeader
	}

	return token, nil
}
