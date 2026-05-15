package storage

import (
	"context"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (*domain.User, error)

	UserByLogin(ctx context.Context, login string) (*domain.User, error)
}

type OrderRepository interface {
	CreateOrder(ctx context.Context, number string, userID int64) error

	OrdersByUser(ctx context.Context, userID int64) ([]domain.Order, error)

	PendingOrders(ctx context.Context, limit int) ([]domain.Order, error)

	ApplyAccrual(ctx context.Context, number string, status domain.OrderStatus, accrual *float64) error
}

type BalanceRepository interface {
	Balance(ctx context.Context, userID int64) (*domain.Balance, error)

	Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error

	WithdrawalsByUser(ctx context.Context, userID int64) ([]domain.Withdrawal, error)
}

type Repository interface {
	UserRepository
	OrderRepository
	BalanceRepository

	Migrate(ctx context.Context) error

	Close()
}
