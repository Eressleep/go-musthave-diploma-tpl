package storage

import "errors"

var (
	ErrNotFound = errors.New("not found")

	ErrLoginTaken = errors.New("login already taken")

	ErrOrderOwnedByOther = errors.New("order owned by another user")

	ErrOrderAlreadyUploaded = errors.New("order already uploaded by user")

	ErrInsufficientFunds = errors.New("insufficient funds")

	ErrOrderNotFound = errors.New("order not found")
)
