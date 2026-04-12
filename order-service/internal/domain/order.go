package domain

import "time"

type Order struct {
	ID         string
	CustomerID string
	ItemName   string
	Amount     int64
	Status     string
	CreatedAt  time.Time
}

type OrderRepository interface {
	Create(order *Order) error
	GetByID(id string) (*Order, error)
	List() ([]Order, error)
	UpdateStatus(id string, status string) error
	SubscribeToOrderUpdates(orderID string) <-chan *Order
}
