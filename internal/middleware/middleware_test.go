package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestWithLogger(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("добавление и извлечение логгера", func(t *testing.T) {
		ctx := WithLogger(t.Context(), logger)
		got := LoggerFromContext(ctx)
		if got != logger {
			t.Error("expected same logger instance")
		}
	})

	t.Run("извлечение из пустого контекста", func(t *testing.T) {
		got := LoggerFromContext(t.Context())
		if got == nil {
			t.Error("expected fallback to global logger")
		}
	})

	t.Run("извлечение неверного типа возвращает глобальный", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), loggerKey, "not-a-logger")
		got := LoggerFromContext(ctx)
		if got == nil {
			t.Error("expected fallback to global logger")
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("перехватывает статус-код", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		rw.WriteHeader(http.StatusCreated)

		if rw.statusCode != http.StatusCreated {
			t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusCreated)
		}
	})

	t.Run("перехватывает количество байт", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		data := []byte("hello world")
		n, err := rw.Write(data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Errorf("written = %d, want %d", n, len(data))
		}
		if rw.bytesWritten != len(data) {
			t.Errorf("bytesWritten = %d, want %d", rw.bytesWritten, len(data))
		}
	})
}

func TestTimeout(t *testing.T) {
	t.Run("запрос завершается до таймаута", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := Timeout(1 * time.Second)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/fast", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("запрос прерывается по таймауту", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(2 * time.Second):
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
			}
		})

		handler := Timeout(50 * time.Millisecond)(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/slow", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Error("expected timeout, but got 200")
		}
	})
}

func TestChain(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("применяет middleware в правильном порядке", func(t *testing.T) {
		var order []string

		mw1 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "mw1_before")
				next.ServeHTTP(w, r)
				order = append(order, "mw1_after")
			})
		}

		mw2 := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "mw2_before")
				next.ServeHTTP(w, r)
				order = append(order, "mw2_after")
			})
		}

		handler := Chain(mw1, mw2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/chain", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		expected := []string{"mw1_before", "mw2_before", "handler", "mw2_after", "mw1_after"}
		if len(order) != len(expected) {
			t.Errorf("order length = %d, want %d", len(order), len(expected))
		}
		for i, v := range expected {
			if i >= len(order) || order[i] != v {
				t.Errorf("order[%d] = %s, want %s", i, order[i], v)
			}
		}
	})

	t.Run("полная цепочка middleware", func(t *testing.T) {
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		chain := Chain(
			RequestID(logger),
			Logging,
			Recovery,
			Timeout(5*time.Second),
		)

		handler := chain(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/full-chain", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID header")
		}
	})
}

func TestGenerateRequestID(t *testing.T) {
	t.Run("генерирует уникальные ID", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateRequestID()
			if ids[id] {
				t.Errorf("duplicate request ID: %s", id)
			}
			ids[id] = true
		}
	})

	t.Run("ID имеет правильный формат", func(t *testing.T) {
		id := generateRequestID()
		if len(id) == 0 {
			t.Error("empty request ID")
		}
		if !strings.Contains(id, "-") {
			t.Error("request ID missing separator")
		}
	})
}

func TestRandomString(t *testing.T) {
	t.Run("генерирует строку правильной длины", func(t *testing.T) {
		for _, n := range []int{0, 1, 8, 16, 32} {
			s := randomString(n)
			if len(s) != n {
				t.Errorf("randomString(%d) length = %d", n, len(s))
			}
		}
	})

	t.Run("содержит только допустимые символы", func(t *testing.T) {
		const allowed = "abcdefghijklmnopqrstuvwxyz0123456789"
		s := randomString(1000)
		for _, c := range s {
			if !strings.ContainsRune(allowed, c) {
				t.Errorf("unexpected character: %c", c)
			}
		}
	})
}
