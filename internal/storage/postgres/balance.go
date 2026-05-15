package postgres

import (
	"context"
	"errors"
	"fmt"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) Balance(ctx context.Context, userID int64) (*domain.Balance, error) {
	b := &domain.Balance{}
	err := r.pool.QueryRow(ctx,
		`SELECT current, withdrawn FROM balances WHERE user_id = $1`,
		userID,
	).Scan(&b.Current, &b.Withdrawn)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("query balance: %w", err)
	}

	return b, nil
}

func (r *Repository) Withdraw(
	ctx context.Context,
	userID int64,
	orderNumber string,
	sum float64,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var current float64
	if err := tx.QueryRow(ctx,
		`SELECT current FROM balances WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&current); err != nil {
		return fmt.Errorf("lock balance: %w", err)
	}

	if current < sum {
		return storage.ErrInsufficientFunds
	}

	if _, err := tx.Exec(ctx,
		`UPDATE balances
		 SET current = current - $1, withdrawn = withdrawn + $1
		 WHERE user_id = $2`,
		sum, userID,
	); err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum)
		 VALUES ($1, $2, $3)`,
		userID, orderNumber, sum,
	); err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) WithdrawalsByUser(
	ctx context.Context,
	userID int64,
) ([]domain.Withdrawal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, order_number, sum, processed_at
		 FROM withdrawals
		 WHERE user_id = $1
		 ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []domain.Withdrawal
	for rows.Next() {
		var w domain.Withdrawal
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.OrderNumber, &w.Sum, &w.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return withdrawals, nil
}
