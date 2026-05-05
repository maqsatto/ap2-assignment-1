package usecase

import (
	"context"
	"fmt"
	"math/rand"
	"payment-service/internal/domain"
	"payment-service/internal/messaging"
	"time"
)

type PaymentEventPublisher interface {
	PublishPaymentCompleted(ctx context.Context, event messaging.PaymentCompletedEvent) error
}

type PaymentUseCase struct {
	repo      domain.PaymentRepository
	publisher PaymentEventPublisher
}

func NewPaymentUseCase(repo domain.PaymentRepository, publisher PaymentEventPublisher) *PaymentUseCase {
	if publisher == nil {
		publisher = messaging.NoopPublisher{}
	}
	return &PaymentUseCase{repo: repo, publisher: publisher}
}

func (uc *PaymentUseCase) ProcessPayment(ctx context.Context, orderID string, amount int64, customerEmail string) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}
	if customerEmail == "" {
		customerEmail = "unknown@example.com"
	}

	var status string
	var transactionID string

	if amount > 100000 {
		status = "Declined"
		transactionID = ""
	} else {
		status = "Authorized"
		transactionID = fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), rand.Intn(9999))
	}

	payment := &domain.Payment{
		ID:            fmt.Sprintf("PAY-%d", time.Now().UnixNano()),
		OrderID:       orderID,
		TransactionID: transactionID,
		Amount:        amount,
		CustomerEmail: customerEmail,
		Status:        status,
		CreatedAt:     time.Now(),
	}

	if err := uc.repo.Create(payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	event := messaging.PaymentCompletedEvent{
		EventID:       payment.ID,
		OrderID:       payment.OrderID,
		Amount:        payment.Amount,
		CustomerEmail: payment.CustomerEmail,
		Status:        payment.Status,
	}
	if err := uc.publisher.PublishPaymentCompleted(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to publish payment completed event: %w", err)
	}

	return payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(orderID string) (*domain.Payment, error) {
	return uc.repo.GetByOrderID(orderID)
}

func (uc *PaymentUseCase) ListPayments() ([]domain.Payment, error) {
	return uc.repo.List()
}
