package postgres

import (
	"context"
	"testing"

	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("успешное создание пользователя", func(t *testing.T) {
		user, err := repo.CreateUser(ctx, "testuser1", "hashed_password_1")
		require.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.Equal(t, "testuser1", user.Login)
		assert.NotZero(t, user.CreatedAt)
	})

	t.Run("попытка создать пользователя с существующим логином", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, "testuser2", "hashed_password_2")
		require.NoError(t, err)

		_, err = repo.CreateUser(ctx, "testuser2", "hashed_password_3")
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrLoginTaken)
	})

	t.Run("баланс нового пользователя инициализирован", func(t *testing.T) {
		user, err := repo.CreateUser(ctx, "testuser3", "hashed_password_4")
		require.NoError(t, err)

		balance, err := repo.Balance(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, 0.0, balance.Current)
		assert.Equal(t, 0.0, balance.Withdrawn)
	})
}

func TestUserByLogin(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("успешный поиск существующего пользователя", func(t *testing.T) {
		created, err := repo.CreateUser(ctx, "finduser", "hashed_password")
		require.NoError(t, err)

		found, err := repo.UserByLogin(ctx, "finduser")
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Login, found.Login)
	})

	t.Run("поиск несуществующего пользователя", func(t *testing.T) {
		_, err := repo.UserByLogin(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
