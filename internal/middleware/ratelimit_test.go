package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("пропускает запросы в пределах лимита", func(t *testing.T) {
		limiter := NewRateLimiter(3, time.Second)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := limiter.Middleware(nextHandler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
			}
		}
	})

	t.Run("блокирует запросы сверх лимита", func(t *testing.T) {
		limiter := NewRateLimiter(2, time.Second)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := limiter.Middleware(nextHandler)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
		}
	})

	t.Run("разные IP считаются отдельно", func(t *testing.T) {
		limiter := NewRateLimiter(1, time.Second)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := limiter.Middleware(nextHandler)

		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "192.168.1.1:1234"
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		if w1.Code != http.StatusOK {
			t.Errorf("IP1 first request: status = %d", w1.Code)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "192.168.1.2:5678"
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		if w2.Code != http.StatusOK {
			t.Errorf("IP2 first request: status = %d", w2.Code)
		}
	})

	t.Run("конкурентные запросы", func(t *testing.T) {
		limiter := NewRateLimiter(100, time.Second)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := limiter.Middleware(nextHandler)

		var wg sync.WaitGroup
		errors := make(chan int, 50)

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					errors <- w.Code
				}
			}()
		}

		wg.Wait()
		close(errors)

		for code := range errors {
			t.Errorf("unexpected status: %d", code)
		}
	})
}
