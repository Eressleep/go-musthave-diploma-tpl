package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestRequestID(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("добавляет X-Request-ID в заголовки ответа", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestID(logger)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("expected X-Request-ID header")
		}
	})

	t.Run("пробрасывает существующий X-Request-ID", func(t *testing.T) {
		existingID := "existing-request-id"

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := w.Header().Get("X-Request-ID")
			if requestID != existingID {
				t.Errorf("requestID = %s, want %s", requestID, existingID)
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestID(logger)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
	})

	t.Run("добавляет логгер в контекст", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqLogger := LoggerFromContext(r.Context())
			if reqLogger == nil {
				t.Error("expected logger in context")
			}
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestID(logger)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
	})
}

func TestLogging(t *testing.T) {
	t.Run("логирует успешный запрос", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		handler := RequestID(logger)(Logging(nextHandler))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("логирует запрос с ошибкой", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		handler := RequestID(logger)(Logging(nextHandler))

		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("логирует 404 запрос", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		handler := RequestID(logger)(Logging(nextHandler))

		req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestRecovery(t *testing.T) {
	t.Run("восстанавливает после паники", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		handler := RequestID(logger)(Recovery(nextHandler))

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("не влияет на обычные запросы", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RequestID(logger)(Recovery(nextHandler))

		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}
