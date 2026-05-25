package domain

import "time"

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusNew, OrderStatusProcessing, OrderStatusInvalid, OrderStatusProcessed:
		return true
	default:
		return false
	}
}

func (s OrderStatus) IsFinal() bool {
	return s == OrderStatusInvalid || s == OrderStatusProcessed
}

type User struct {
	ID           int64     `json:"-"`
	Login        string    `json:"login"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
}

type Order struct {
	Number     string      `json:"number"`
	UserID     int64       `json:"-"`
	Status     OrderStatus `json:"status"`
	Accrual    *float64    `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at"`
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdrawal struct {
	ID          int64     `json:"-"`
	UserID      int64     `json:"-"`
	OrderNumber string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
