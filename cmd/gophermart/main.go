package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Eressleep/go-musthave-diploma-tpl/internal/accrual"
	"Eressleep/go-musthave-diploma-tpl/internal/auth"
	"Eressleep/go-musthave-diploma-tpl/internal/config"
	"Eressleep/go-musthave-diploma-tpl/internal/handlers"
	"Eressleep/go-musthave-diploma-tpl/internal/storage/postgres"

	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("configuration loaded",
		zap.String("run_address", cfg.RunAddress),
		zap.String("database_uri", cfg.MaskedDatabaseURI()),
		zap.String("accrual_address", cfg.AccrualSystemAddress),
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	repo, err := postgres.New(ctx, cfg.DatabaseURI)
	if err != nil {
		return fmt.Errorf("connect storage: %w", err)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	authMgr := auth.NewManager(cfg.JWTSecret)
	h := handlers.New(repo, authMgr, logger)

	accrualClient := accrual.NewClient(cfg.AccrualSystemAddress)
	worker := accrual.NewWorker(repo, accrualClient, logger)
	go worker.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           h.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", zap.String("addr", cfg.RunAddress))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
	return nil
}
