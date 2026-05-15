package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockTokenParser struct {
	userID int64
	err    error
}

func (m *mockTokenParser) Parse(tokenString string) (int64, error) {
	return m.userID, m.err
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		parser     *mockTokenParser
		wantStatus int
		wantUserID int64
	}{
		{
			name:       "успешная аутентификация",
			token:      "Bearer valid-token",
			parser:     &mockTokenParser{userID: 42},
			wantStatus: http.StatusOK,
			wantUserID: 42,
		},
		{
			name:       "отсутствует заголовок Authorization",
			token:      "",
			parser:     &mockTokenParser{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "неверный формат токена (без Bearer)",
			token:      "invalid-token",
			parser:     &mockTokenParser{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "пустой токен после Bearer",
			token:      "Bearer ",
			parser:     &mockTokenParser{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "невалидный токен",
			token:      "Bearer expired-token",
			parser:     &mockTokenParser{err: http.ErrNoCookie},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userID, ok := UserIDFromContext(r.Context())
				if !ok && tt.wantUserID != 0 {
					t.Error("expected userID in context")
				}
				if ok && userID != tt.wantUserID {
					t.Errorf("userID = %d, want %d", userID, tt.wantUserID)
				}
				w.WriteHeader(http.StatusOK)
			})

			middleware := AuthMiddleware(tt.parser)
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestWithUserID(t *testing.T) {
	t.Run("добавление и извлечение userID", func(t *testing.T) {
		ctx := WithUserID(t.Context(), 123)
		userID, ok := UserIDFromContext(ctx)
		if !ok {
			t.Error("expected userID in context")
		}
		if userID != 123 {
			t.Errorf("userID = %d, want 123", userID)
		}
	})

	t.Run("извлечение из пустого контекста", func(t *testing.T) {
		_, ok := UserIDFromContext(t.Context())
		if ok {
			t.Error("expected no userID in empty context")
		}
	})

	t.Run("извлечение неверного типа", func(t *testing.T) {
		ctx := t.Context()
		ctx = context.WithValue(ctx, userIDKey, "not-an-int")
		_, ok := UserIDFromContext(ctx)
		if ok {
			t.Error("expected false for wrong type")
		}
	})
}
