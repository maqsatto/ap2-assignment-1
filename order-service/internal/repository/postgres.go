package repository

import (
	"database/sql"
	"order-service/internal/domain"
	"time"
)

type OrderRepo struct {
	db *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(order *domain.Order) error {
	_, err := r.db.Exec(
		"INSERT INTO orders (id, customer_id, item_name, amount, status, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		order.ID, order.CustomerID, order.ItemName, order.Amount, order.Status, order.CreatedAt,
	)
	return err
}

func (r *OrderRepo) GetByID(id string) (*domain.Order, error) {
	o := &domain.Order{}
	err := r.db.QueryRow(
		"SELECT id, customer_id, item_name, amount, status, created_at FROM orders WHERE id = $1",
		id,
	).Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *OrderRepo) List() ([]domain.Order, error) {
	rows, err := r.db.Query(
		"SELECT id, customer_id, item_name, amount, status, created_at FROM orders ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *OrderRepo) UpdateStatus(id string, status string) error {
	_, err := r.db.Exec("UPDATE orders SET status = $1 WHERE id = $2", status, id)
	return err
}

// SubscribeToOrderUpdates returns a channel that emits order updates when the order status changes in the DB
func (r *OrderRepo) SubscribeToOrderUpdates(orderID string) <-chan *domain.Order {
	ch := make(chan *domain.Order, 10)

	go func() {
		defer close(ch)

		var lastStatus string
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				order, err := r.GetByID(orderID)
				if err != nil {
					// Order doesn't exist, stop streaming
					return
				}

				// Only send update if status has changed
				if order.Status != lastStatus {
					lastStatus = order.Status
					ch <- order
				}
			}
		}
	}()

	return ch
}
