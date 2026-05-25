package postgres

import (
	"context"
	"testing"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"
	"Eressleep/go-musthave-diploma-tpl/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrder(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()
	userID := createTestUser(t, repo, "orderuser")

	t.Run("успешное создание заказа", func(t *testing.T) {
		err := repo.CreateOrder(ctx, "12345678903", userID)
		require.NoError(t, err)
	})

	t.Run("дубликат заказа тем же пользователем", func(t *testing.T) {
		err := repo.CreateOrder(ctx, "12345678903", userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrOrderAlreadyUploaded)
	})

	t.Run("дубликат заказа другим пользователем", func(t *testing.T) {
		otherUserID := createTestUser(t, repo, "otheruser")
		err := repo.CreateOrder(ctx, "12345678903", otherUserID)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrOrderOwnedByOther)
	})

	t.Run("создание нескольких заказов для одного пользователя", func(t *testing.T) {
		orders := []string{
			"49927398716",
			"1234567812345670",
			"4532015112830366",
		}

		for _, orderNum := range orders {
			err := repo.CreateOrder(ctx, orderNum, userID)
			require.NoError(t, err, "failed to create order %s", orderNum)
		}
	})
}

func TestOrdersByUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("получение заказов пользователя", func(t *testing.T) {
		userID := createTestUser(t, repo, "listuser")

		orders := []string{"49927398716", "1234567812345670"}
		for _, num := range orders {
			err := repo.CreateOrder(ctx, num, userID)
			require.NoError(t, err)
		}

		result, err := repo.OrdersByUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, result, 2)

		for _, order := range result {
			assert.Equal(t, domain.OrderStatusNew, order.Status)
			assert.Nil(t, order.Accrual)
		}
	})

	t.Run("получение заказов несуществующего пользователя", func(t *testing.T) {
		result, err := repo.OrdersByUser(ctx, 99999)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("проверка сортировки заказов по времени", func(t *testing.T) {
		userID := createTestUser(t, repo, "sortuser")

		first := "49927398716"
		second := "1234567812345670"

		require.NoError(t, repo.CreateOrder(ctx, first, userID))
		require.NoError(t, repo.CreateOrder(ctx, second, userID))

		result, err := repo.OrdersByUser(ctx, userID)
		require.NoError(t, err)
		require.Len(t, result, 2)

		assert.True(t, result[0].UploadedAt.After(result[1].UploadedAt) ||
			result[0].UploadedAt.Equal(result[1].UploadedAt))
	})
}

func TestPendingOrders(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("получение ожидающих заказов", func(t *testing.T) {
		userID := createTestUser(t, repo, "pendinguser")

		require.NoError(t, repo.CreateOrder(ctx, "49927398716", userID))
		require.NoError(t, repo.CreateOrder(ctx, "1234567812345670", userID))

		pending, err := repo.PendingOrders(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, pending, 2)

		for _, order := range pending {
			assert.Equal(t, domain.OrderStatusNew, order.Status)
		}
	})

	t.Run("ограничение количества заказов", func(t *testing.T) {
		pending, err := repo.PendingOrders(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, pending, 1)
	})
}

func TestApplyAccrual(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := createTestRepository(t, db.dsn)
	defer repo.Close()

	ctx := context.Background()

	t.Run("начисление баллов за обработанный заказ", func(t *testing.T) {
		userID := createTestUser(t, repo, "accrualuser")
		orderNum := "49927398716"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))

		accrual := 500.0
		err := repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusProcessed, &accrual)
		require.NoError(t, err)

		orders, err := repo.OrdersByUser(ctx, userID)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, domain.OrderStatusProcessed, orders[0].Status)
		assert.Equal(t, &accrual, orders[0].Accrual)

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 500.0, balance.Current)
		assert.Equal(t, 0.0, balance.Withdrawn)
	})

	t.Run("обновление статуса без начисления", func(t *testing.T) {
		userID := createTestUser(t, repo, "statususer")
		orderNum := "1234567812345670"

		require.NoError(t, repo.CreateOrder(ctx, orderNum, userID))

		err := repo.ApplyAccrual(ctx, orderNum, domain.OrderStatusInvalid, nil)
		require.NoError(t, err)

		orders, err := repo.OrdersByUser(ctx, userID)
		require.NoError(t, err)
		require.Len(t, orders, 1)
		assert.Equal(t, domain.OrderStatusInvalid, orders[0].Status)
		assert.Nil(t, orders[0].Accrual)

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 0.0, balance.Current)
	})

	t.Run("множественные начисления одному пользователю", func(t *testing.T) {
		userID := createTestUser(t, repo, "multiuser")

		orders := []struct {
			num     string
			accrual float64
		}{
			{"49927398716", 100.0},
			{"1234567812345670", 250.0},
			{"4532015112830366", 150.0},
		}

		for _, o := range orders {
			require.NoError(t, repo.CreateOrder(ctx, o.num, userID))
		}

		for _, o := range orders {
			accrual := o.accrual
			err := repo.ApplyAccrual(ctx, o.num, domain.OrderStatusProcessed, &accrual)
			require.NoError(t, err)
		}

		balance, err := repo.Balance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 500.0, balance.Current)
	})
}
