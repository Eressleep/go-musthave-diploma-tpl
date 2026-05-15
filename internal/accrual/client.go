package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type AccrualStatus string

const (
	StatusRegistered AccrualStatus = "REGISTERED"
	StatusInvalid    AccrualStatus = "INVALID"
	StatusProcessing AccrualStatus = "PROCESSING"
	StatusProcessed  AccrualStatus = "PROCESSED"
)

type Response struct {
	Order   string        `json:"order"`
	Status  AccrualStatus `json:"status"`
	Accrual *float64      `json:"accrual,omitempty"`
}

var ErrNotRegistered = errors.New("order not registered in accrual system")

type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	return fmt.Sprintf("accrual rate limited, retry after %s", e.RetryAfter)
}

var ErrCircuitOpen = errors.New("circuit breaker is open")

type Client struct {
	baseURL      string
	httpClient   *http.Client
	circuitOpen  atomic.Bool
	failureCount atomic.Int64
	lastFailure  atomic.Int64
	maxFailures  int64
	resetTimeout time.Duration
}

type ClientOption func(*Client)

func WithMaxFailures(n int64) ClientOption {
	return func(c *Client) {
		c.maxFailures = n
	}
}

func WithResetTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.resetTimeout = d
	}
}

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		},
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) GetOrder(ctx context.Context, number string) (*Response, error) {
	if c.circuitOpen.Load() {
		if time.Now().UnixNano()-c.lastFailure.Load() < int64(c.resetTimeout) {
			return nil, ErrCircuitOpen
		}
		c.circuitOpen.Store(false)
		c.failureCount.Store(0)
	}

	resp, err := c.doRequest(ctx, number)
	if err != nil {
		c.recordFailure()
		return nil, err
	}

	c.failureCount.Store(0)
	return resp, nil
}

func (c *Client) doRequest(ctx context.Context, number string) (*Response, error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, number)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var out Response
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return &out, nil

	case http.StatusNoContent:
		return nil, ErrNotRegistered

	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &ErrRateLimited{RetryAfter: retryAfter}

	default:
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func (c *Client) recordFailure() {
	count := c.failureCount.Add(1)
	if count >= c.maxFailures {
		c.circuitOpen.Store(true)
		c.lastFailure.Store(time.Now().UnixNano())
	}
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 60 * time.Second
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 60 * time.Second
}
