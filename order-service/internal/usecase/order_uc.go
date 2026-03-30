package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"order-service/internal/domain"
	"time"
)

type PaymentClient interface {
	ProcessPayment(orderID string, amount int64) (status string, err error)
}

type PaymentHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPaymentHTTPClient(baseURL string) *PaymentHTTPClient {
	return &PaymentHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

type paymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type paymentResponse struct {
	Status string `json:"status"`
}

func (c *PaymentHTTPClient) ProcessPayment(orderID string, amount int64) (string, error) {
	body := paymentRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payment request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/payments", "application/json", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("payment service unavailable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read payment response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("payment service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var payResp paymentResponse
	if err := json.Unmarshal(respBody, &payResp); err != nil {
		return "", fmt.Errorf("failed to parse payment response: %w", err)
	}

	return payResp.Status, nil
}

type OrderUseCase struct {
	repo          domain.OrderRepository
	paymentClient PaymentClient
}

var ErrPaymentServiceUnavailable = errors.New("payment service unavailable")

func NewOrderUseCase(repo domain.OrderRepository, paymentClient PaymentClient) *OrderUseCase {
	return &OrderUseCase{
		repo:          repo,
		paymentClient: paymentClient,
	}
}

func (uc *OrderUseCase) CreateOrder(customerID, itemName string, amount int64) (*domain.Order, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}

	order := &domain.Order{
		ID:         fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		CustomerID: customerID,
		ItemName:   itemName,
		Amount:     amount,
		Status:     "Pending",
		CreatedAt:  time.Now(),
	}

	if err := uc.repo.Create(order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	paymentStatus, err := uc.paymentClient.ProcessPayment(order.ID, order.Amount)
	if err != nil {
		uc.repo.UpdateStatus(order.ID, "Failed")
		order.Status = "Failed"
		return nil, fmt.Errorf("%w: %v", ErrPaymentServiceUnavailable, err)
	}

	if paymentStatus == "Authorized" {
		uc.repo.UpdateStatus(order.ID, "Paid")
		order.Status = "Paid"
	} else {
		uc.repo.UpdateStatus(order.ID, "Failed")
		order.Status = "Failed"
	}

	return order, nil
}

func (uc *OrderUseCase) GetOrder(id string) (*domain.Order, error) {
	return uc.repo.GetByID(id)
}

func (uc *OrderUseCase) ListOrders() ([]domain.Order, error) {
	return uc.repo.List()
}

func (uc *OrderUseCase) CancelOrder(id string) error {
	order, err := uc.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("order not found")
	}

	if order.Status != "Pending" {
		return fmt.Errorf("only pending orders can be cancelled")
	}

	return uc.repo.UpdateStatus(id, "Cancelled")
}
