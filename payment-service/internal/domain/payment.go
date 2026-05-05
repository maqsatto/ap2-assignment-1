package domain

import "time"

type Payment struct {
	ID            string
	OrderID       string
	TransactionID string
	Amount        int64
	CustomerEmail string
	Status        string
	CreatedAt     time.Time
}

type PaymentRepository interface {
	Create(payment *Payment) error
	GetByOrderID(orderID string) (*Payment, error)
	List() ([]Payment, error)
}
