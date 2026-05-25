package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"Eressleep/go-musthave-diploma-tpl/internal/auth"
	"Eressleep/go-musthave-diploma-tpl/internal/domain"
)

// mockStorage реализует storage.Repository для тестов.
type mockStorage struct {
	createUserFn        func(ctx context.Context, login, passwordHash string) (*domain.User, error)
	userByLoginFn       func(ctx context.Context, login string) (*domain.User, error)
	createOrderFn       func(ctx context.Context, number string, userID int64) error
	ordersByUserFn      func(ctx context.Context, userID int64) ([]domain.Order, error)
	balanceFn           func(ctx context.Context, userID int64) (*domain.Balance, error)
	withdrawFn          func(ctx context.Context, userID int64, orderNumber string, sum float64) error
	withdrawalsByUserFn func(ctx context.Context, userID int64) ([]domain.Withdrawal, error)
}

func (m *mockStorage) CreateUser(ctx context.Context, login, passwordHash string) (*domain.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, login, passwordHash)
	}
	return &domain.User{ID: 1, Login: login}, nil
}

func (m *mockStorage) UserByLogin(ctx context.Context, login string) (*domain.User, error) {
	if m.userByLoginFn != nil {
		return m.userByLoginFn(ctx, login)
	}
	return &domain.User{ID: 1, Login: login, PasswordHash: "$2a$10$test"}, nil
}

func (m *mockStorage) CreateOrder(ctx context.Context, number string, userID int64) error {
	if m.createOrderFn != nil {
		return m.createOrderFn(ctx, number, userID)
	}
	return nil
}

func (m *mockStorage) OrdersByUser(ctx context.Context, userID int64) ([]domain.Order, error) {
	if m.ordersByUserFn != nil {
		return m.ordersByUserFn(ctx, userID)
	}
	return []domain.Order{}, nil
}

func (m *mockStorage) PendingOrders(ctx context.Context, limit int) ([]domain.Order, error) {
	return nil, nil
}

func (m *mockStorage) ApplyAccrual(ctx context.Context, number string, status domain.OrderStatus, accrual *float64) error {
	return nil
}

func (m *mockStorage) Balance(ctx context.Context, userID int64) (*domain.Balance, error) {
	if m.balanceFn != nil {
		return m.balanceFn(ctx, userID)
	}
	return &domain.Balance{Current: 100, Withdrawn: 50}, nil
}

func (m *mockStorage) Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error {
	if m.withdrawFn != nil {
		return m.withdrawFn(ctx, userID, orderNumber, sum)
	}
	return nil
}

func (m *mockStorage) WithdrawalsByUser(ctx context.Context, userID int64) ([]domain.Withdrawal, error) {
	if m.withdrawalsByUserFn != nil {
		return m.withdrawalsByUserFn(ctx, userID)
	}
	return []domain.Withdrawal{}, nil
}

func (m *mockStorage) Migrate(ctx context.Context) error { return nil }
func (m *mockStorage) Close()                            {}

// authRequest выполняет запрос с токеном аутентификации.
func authRequest(method, path, body string, userID int64, authMgr *auth.Manager) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID > 0 {
		token, _ := authMgr.Issue(userID)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}
