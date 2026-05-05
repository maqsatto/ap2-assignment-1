package repository

import (
	"database/sql"
	"payment-service/internal/domain"
)

type PaymentRepo struct {
	db *sql.DB
}

func NewPaymentRepo(db *sql.DB) (*PaymentRepo, error) {
	repo := &PaymentRepo{db: db}
	if err := repo.ensureSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *PaymentRepo) ensureSchema() error {
	_, err := r.db.Exec(`
		ALTER TABLE payments
		ADD COLUMN IF NOT EXISTS customer_email VARCHAR(255) NOT NULL DEFAULT 'unknown@example.com'
	`)
	return err
}

func (r *PaymentRepo) Create(payment *domain.Payment) error {
	_, err := r.db.Exec(
		"INSERT INTO payments (id, order_id, transaction_id, amount, customer_email, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		payment.ID, payment.OrderID, payment.TransactionID, payment.Amount, payment.CustomerEmail, payment.Status, payment.CreatedAt,
	)
	return err
}

func (r *PaymentRepo) GetByOrderID(orderID string) (*domain.Payment, error) {
	p := &domain.Payment{}
	var txID sql.NullString
	err := r.db.QueryRow(
		"SELECT id, order_id, transaction_id, amount, customer_email, status, created_at FROM payments WHERE order_id = $1",
		orderID,
	).Scan(&p.ID, &p.OrderID, &txID, &p.Amount, &p.CustomerEmail, &p.Status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	if txID.Valid {
		p.TransactionID = txID.String
	}
	return p, nil
}

func (r *PaymentRepo) List() ([]domain.Payment, error) {
	rows, err := r.db.Query(
		"SELECT id, order_id, transaction_id, amount, customer_email, status, created_at FROM payments ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0)
	for rows.Next() {
		var p domain.Payment
		var txID sql.NullString
		if err := rows.Scan(&p.ID, &p.OrderID, &txID, &p.Amount, &p.CustomerEmail, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		if txID.Valid {
			p.TransactionID = txID.String
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}
