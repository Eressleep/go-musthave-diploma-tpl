package postgres

import (
	"context"
	"testing"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalance(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("баланс нового пользователя", func(t *testing.T) {
		user, err := repo.CreateUser(ctx, "balanceuser", "hash")
		require.NoError(t, err)

		balance, err := repo.Balance(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, 0.0, balance.Current)
		assert.Equal(t, 0.0, balance.Withdrawn)
	})

	t.Run("баланс несуществующего пользователя", func(t *testing.T) {
		_, err := repo.Balance(ctx, 99999)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("баланс после начислений и списаний", func(t *testing.T) {
		userID := createTestUser(t, repo, "fullflowuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 1000.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 1000.0, balance.Current)

		require.NoError(t, repo.Withdraw(ctx, userID, "2377225624", 300.0))

		balance, err = repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 700.0, balance.Current)
		assert.Equal(t, 300.0, balance.Withdrawn)
	})
}

func TestWithdraw(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("успешное списание средств", func(t *testing.T) {
		userID := createTestUser(t, repo, "withdrawuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 500.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))

		err := repo.Withdraw(ctx, userID, "2377225624", 200.0)
		require.NoError(t, err)

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 300.0, balance.Current)
		assert.Equal(t, 200.0, balance.Withdrawn)
	})

	t.Run("списание при недостаточном балансе", func(t *testing.T) {
		userID := createTestUser(t, repo, "pooruser")

		err := repo.Withdraw(ctx, userID, "2377225624", 100.0)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrInsufficientFunds)
	})

	t.Run("списание нулевой суммы", func(t *testing.T) {
		userID := createTestUser(t, repo, "zerouser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 100.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))

		err := repo.Withdraw(ctx, userID, "00000000000", 0.0)
		require.NoError(t, err)
	})

	t.Run("множественные списания", func(t *testing.T) {
		userID := createTestUser(t, repo, "multiswipeuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 1000.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))

		withdrawals := []struct {
			order string
			sum   float64
		}{
			{"11111111111", 200.0},
			{"22222222222", 300.0},
			{"33333333333", 100.0},
		}

		for _, w := range withdrawals {
			require.NoError(t, repo.Withdraw(ctx, userID, w.order, w.sum))
		}

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 400.0, balance.Current)
		assert.Equal(t, 600.0, balance.Withdrawn)
	})
}

func TestWithdrawalsByUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("получение истории списаний", func(t *testing.T) {
		userID := createTestUser(t, repo, "historyuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 1000.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))

		require.NoError(t, repo.Withdraw(ctx, userID, "11111111111", 100.0))
		require.NoError(t, repo.Withdraw(ctx, userID, "22222222222", 200.0))

		withdrawals, err := repo.WithdrawalsByUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, withdrawals, 2)

		assert.True(t, withdrawals[0].ProcessedAt.After(withdrawals[1].ProcessedAt) ||
			withdrawals[0].ProcessedAt.Equal(withdrawals[1].ProcessedAt))
	})

	t.Run("пустая история списаний", func(t *testing.T) {
		userID := createTestUser(t, repo, "emptyuser")

		withdrawals, err := repo.WithdrawalsByUser(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, withdrawals)
	})

	t.Run("проверка данных списания", func(t *testing.T) {
		userID := createTestUser(t, repo, "checkuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))
		accrual := 500.0
		require.NoError(t, repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual))
		require.NoError(t, repo.Withdraw(ctx, userID, "99999999999", 150.0))

		withdrawals, err := repo.WithdrawalsByUser(ctx, userID)
		require.NoError(t, err)
		require.Len(t, withdrawals, 1)

		assert.Equal(t, "99999999999", withdrawals[0].OrderNumber)
		assert.Equal(t, 150.0, withdrawals[0].Sum)
		assert.NotZero(t, withdrawals[0].ProcessedAt)
	})
}
