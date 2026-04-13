package repository

import (
	"database/sql"
	"order-service/internal/domain"
	"time"
)

type OrderRepo struct {
	db *sql.DB
}

func NewOrderRepo(db *sql.DB) (*OrderRepo, error) {
	repo := &OrderRepo{db: db}
	if err := repo.ensureSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *OrderRepo) ensureSchema() error {
	_, err := r.db.Exec(`
		ALTER TABLE orders
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	`)
	return err
}

func (r *OrderRepo) Create(order *domain.Order) error {
	_, err := r.db.Exec(
		"INSERT INTO orders (id, customer_id, item_name, amount, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		order.ID, order.CustomerID, order.ItemName, order.Amount, order.Status, order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (r *OrderRepo) GetByID(id string) (*domain.Order, error) {
	o := &domain.Order{}
	err := r.db.QueryRow(
		"SELECT id, customer_id, item_name, amount, status, created_at, updated_at FROM orders WHERE id = $1",
		id,
	).Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *OrderRepo) List() ([]domain.Order, error) {
	rows, err := r.db.Query(
		"SELECT id, customer_id, item_name, amount, status, created_at, updated_at FROM orders ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
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
	res, err := r.db.Exec("UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// SubscribeToOrderUpdates returns a channel that emits order updates when the order status changes in the DB
func (r *OrderRepo) SubscribeToOrderUpdates(orderID string) <-chan *domain.Order {
	ch := make(chan *domain.Order, 10)

	go func() {
		defer close(ch)

		var lastUpdatedAt time.Time
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

				// Only send update if the row was actually modified in the DB
				if order.UpdatedAt.IsZero() || !order.UpdatedAt.Equal(lastUpdatedAt) {
					lastUpdatedAt = order.UpdatedAt
					ch <- order
				}
			}
		}
	}()

	return ch
}
