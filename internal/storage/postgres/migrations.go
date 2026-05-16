package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migration struct {
	Name    string
	UpSQL   string
	DownSQL string
}

func (r *Repository) MigrateUp(ctx context.Context) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	if _, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, m := range migrations {
		if err := r.applyMigration(ctx, m); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) applyMigration(ctx context.Context, m Migration) error {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`,
		m.Name,
	).Scan(&exists)

	if err != nil || exists {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
		return fmt.Errorf("apply %s: %w", m.Name, err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (name) VALUES ($1)`,
		m.Name,
	); err != nil {
		return fmt.Errorf("record %s: %w", m.Name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", m.Name, err)
	}

	return nil
}

func (r *Repository) MigrateDown(ctx context.Context) error {
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	var lastName string
	err = r.pool.QueryRow(ctx,
		`SELECT name FROM schema_migrations ORDER BY name DESC LIMIT 1`,
	).Scan(&lastName)

	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil
		}
		return fmt.Errorf("get last migration: %w", err)
	}

	for _, m := range migrations {
		if m.Name != lastName {
			continue
		}

		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, m.DownSQL); err != nil {
			return fmt.Errorf("rollback %s: %w", m.Name, err)
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM schema_migrations WHERE name = $1`,
			m.Name,
		); err != nil {
			return fmt.Errorf("delete record %s: %w", m.Name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", m.Name, err)
		}
		break
	}

	return nil
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		parts := strings.SplitN(string(content), "-- +migrate Down", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration format: %s", entry.Name())
		}

		upSQL := strings.TrimSpace(strings.TrimPrefix(parts[0], "-- +migrate Up"))
		downSQL := strings.TrimSpace(parts[1])

		if upSQL == "" || downSQL == "" {
			return nil, fmt.Errorf("empty SQL in: %s", entry.Name())
		}

		migrations = append(migrations, Migration{
			Name:    entry.Name(),
			UpSQL:   upSQL,
			DownSQL: downSQL,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

func (r *Repository) Migrate(ctx context.Context) error {
	return r.MigrateUp(ctx)
}
