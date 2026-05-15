package postgres

import (
	"context"
	"errors"
	"fmt"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateOrder(ctx context.Context, number string, userID int64) error {
	var existingUserID int64
	err := r.pool.QueryRow(ctx,
		`SELECT user_id FROM orders WHERE number = $1`,
		number,
	).Scan(&existingUserID)

	if err == nil {
		if existingUserID == userID {
			return storage.ErrOrderAlreadyUploaded
		}
		return storage.ErrOrderOwnedByOther
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check order: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO orders (number, user_id, status)
		 VALUES ($1, $2, $3)`,
		number, userID, string(domain.OrderStatusNew),
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	return nil
}

func (r *Repository) OrdersByUser(ctx context.Context, userID int64) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT number, user_id, status, accrual, uploaded_at
		 FROM orders
		 WHERE user_id = $1
		 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		var status string
		if err := rows.Scan(
			&o.Number, &o.UserID, &status, &o.Accrual, &o.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.Status = domain.OrderStatus(status)
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if orders == nil {
		orders = []domain.Order{}
	}

	return orders, nil
}

func (r *Repository) PendingOrders(ctx context.Context, limit int) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT number, user_id, status, accrual, uploaded_at
		 FROM orders
		 WHERE status IN ('NEW', 'PROCESSING')
		 ORDER BY uploaded_at ASC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		var status string
		if err := rows.Scan(
			&o.Number, &o.UserID, &status, &o.Accrual, &o.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.Status = domain.OrderStatus(status)
		orders = append(orders, o)
	}

	return orders, rows.Err()
}

func (r *Repository) ApplyAccrual(
	ctx context.Context,
	number string,
	status domain.OrderStatus,
	accrual *float64,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID int64
	if err := tx.QueryRow(ctx,
		`SELECT user_id FROM orders WHERE number = $1 FOR UPDATE`,
		number,
	).Scan(&userID); err != nil {
		return fmt.Errorf("lock order: %w", err)
	}

	if accrual != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
			string(status), *accrual, number,
		); err != nil {
			return fmt.Errorf("update order: %w", err)
		}

		if *accrual > 0 {
			if _, err := tx.Exec(ctx,
				`UPDATE balances SET current = current + $1 WHERE user_id = $2`,
				*accrual, userID,
			); err != nil {
				return fmt.Errorf("update balance: %w", err)
			}
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE orders SET status = $1 WHERE number = $2`,
			string(status), number,
		); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
