package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testDB struct {
	container *postgres.PostgresContainer
	dsn       string
}

func setupTestDB(t *testing.T) (*testDB, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("gophermart_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return &testDB{container: pgContainer, dsn: dsn}, cleanup
}

func createTestRepository(t *testing.T, dsn string) *Repository {
	t.Helper()

	ctx := context.Background()
	repo, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	if err := repo.Migrate(ctx); err != nil {
		repo.Close()
		t.Fatalf("failed to apply migrations: %v", err)
	}

	return repo
}

func createTestUser(t *testing.T, repo *Repository, login string) int64 {
	t.Helper()

	ctx := context.Background()
	user, err := repo.CreateUser(ctx, login, "$2a$10$hashedpassword")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user.ID
}
