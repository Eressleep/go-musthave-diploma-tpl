package accrual

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"Eressleep/go-musthave-diploma-tpl/internal/domain"

	"go.uber.org/zap"
)

type WorkerStorage interface {
	PendingOrders(ctx context.Context, limit int) ([]domain.Order, error)
	ApplyAccrual(ctx context.Context, number string, status domain.OrderStatus, accrual *float64) error
}

const (
	pollBatchSize = 50
	workerCount   = 4
	pollInterval  = 2 * time.Second
)

type Worker struct {
	storage    WorkerStorage
	client     *Client
	logger     *zap.Logger
	pauseUntil atomic.Int64
}

func NewWorker(s WorkerStorage, c *Client, logger *zap.Logger) *Worker {
	return &Worker{
		storage: s,
		client:  c,
		logger:  logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	w.logger.Info("accrual worker started",
		zap.Int("workers", workerCount),
		zap.Duration("interval", pollInterval),
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("accrual worker stopped")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	if until := w.pauseUntil.Load(); until > 0 {
		if time.Now().UnixNano() < until {
			return
		}
		w.pauseUntil.Store(0)
	}

	orders, err := w.storage.PendingOrders(ctx, pollBatchSize)
	if err != nil {
		w.logger.Error("fetch pending orders", zap.Error(err))
		return
	}

	if len(orders) == 0 {
		return
	}

	w.logger.Debug("processing batch", zap.Int("count", len(orders)))

	jobs := make(chan domain.Order, len(orders))
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex

	for i := range workerCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for order := range jobs {
				if err := w.processOrder(ctx, order); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}(i)
	}

	for _, order := range orders {
		jobs <- order
	}
	close(jobs)

	wg.Wait()

	if len(errs) > 0 {
		w.logger.Error("batch processing completed with errors",
			zap.Int("total", len(orders)),
			zap.Int("errors", len(errs)),
			zap.Error(errors.Join(errs...)),
		)
	}
}

func (w *Worker) processOrder(ctx context.Context, order domain.Order) error {
	resp, err := w.client.GetOrder(ctx, order.Number)
	if err != nil {
		return w.handleOrderError(err, order.Number)
	}

	newStatus := mapStatus(resp.Status)

	if newStatus == order.Status && resp.Accrual == nil {
		return nil
	}

	w.logger.Debug("updating order",
		zap.String("number", order.Number),
		zap.String("status", string(newStatus)),
	)

	return w.storage.ApplyAccrual(ctx, order.Number, newStatus, resp.Accrual)
}

func (w *Worker) handleOrderError(err error, orderNumber string) error {
	var rl *ErrRateLimited
	switch {
	case errors.As(err, &rl):
		until := time.Now().Add(rl.RetryAfter).UnixNano()
		w.pauseUntil.Store(until)
		w.logger.Warn("rate limited",
			zap.Duration("retry_after", rl.RetryAfter),
		)
		return nil
	case errors.Is(err, ErrNotRegistered):
		return nil
	default:
		w.logger.Error("accrual request failed",
			zap.String("number", orderNumber),
			zap.Error(err),
		)
		return err
	}
}

func mapStatus(s AccrualStatus) domain.OrderStatus {
	switch s {
	case StatusProcessed:
		return domain.OrderStatusProcessed
	case StatusInvalid:
		return domain.OrderStatusInvalid
	case StatusProcessing, StatusRegistered:
		return domain.OrderStatusProcessing
	default:
		return domain.OrderStatusNew
	}
}
