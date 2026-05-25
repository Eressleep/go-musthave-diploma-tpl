package postgres

import (
	"context"
	"errors"
	"fmt"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationCode = "23505"

func (r *Repository) CreateUser(ctx context.Context, login, passwordHash string) (*domain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	u := &domain.User{Login: login, PasswordHash: passwordHash}
	err = tx.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2)
		 RETURNING id, created_at`,
		login, passwordHash,
	).Scan(&u.ID, &u.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return nil, storage.ErrLoginTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO balances (user_id, current, withdrawn) VALUES ($1, 0, 0)`,
		u.ID,
	); err != nil {
		return nil, fmt.Errorf("init balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return u, nil
}

func (r *Repository) UserByLogin(ctx context.Context, login string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, created_at FROM users WHERE login = $1`,
		login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.CreatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	return u, nil
}
