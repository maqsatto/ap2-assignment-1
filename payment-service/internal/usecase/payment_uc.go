package usecase

import (
	"fmt"
	"math/rand"
	"payment-service/internal/domain"
	"time"
)

type PaymentUseCase struct {
	repo domain.PaymentRepository
}

func NewPaymentUseCase(repo domain.PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{repo: repo}
}

func (uc *PaymentUseCase) ProcessPayment(orderID string, amount int64) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
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
		Status:        status,
		CreatedAt:     time.Now(),
	}

	if err := uc.repo.Create(payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	return payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(orderID string) (*domain.Payment, error) {
	return uc.repo.GetByOrderID(orderID)
}

func (uc *PaymentUseCase) ListPayments() ([]domain.Payment, error) {
	return uc.repo.List()
}
