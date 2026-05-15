package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("успешное подключение к базе данных", func(t *testing.T) {
		repo, err := New(context.Background(), db.dsn)
		require.NoError(t, err)
		defer repo.Close()

		assert.NotNil(t, repo.pool)
	})

	t.Run("повторное применение миграций безопасно", func(t *testing.T) {
		repo, err := New(context.Background(), db.dsn)
		require.NoError(t, err)
		defer repo.Close()

		require.NoError(t, repo.Migrate(context.Background()))

		require.NoError(t, repo.Migrate(context.Background()))
	})

	t.Run("пул соединений работает", func(t *testing.T) {
		repo, err := New(context.Background(), db.dsn)
		require.NoError(t, err)
		defer repo.Close()

		pool := repo.Pool()
		assert.NotNil(t, pool)

		var result int
		err = pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	})
}
