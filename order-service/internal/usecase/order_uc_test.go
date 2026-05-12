package usecase

import (
	"database/sql"
	"testing"
	"time"

	"order-service/internal/domain"
)

type fakeOrderRepo struct {
	orders        map[string]*domain.Order
	getCalls      int
	updateStatus  string
	updateOrderID string
}

func (r *fakeOrderRepo) Create(order *domain.Order) error {
	r.orders[order.ID] = order
	return nil
}

func (r *fakeOrderRepo) GetByID(id string) (*domain.Order, error) {
	r.getCalls++
	order, ok := r.orders[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := *order
	return &copy, nil
}

func (r *fakeOrderRepo) List() ([]domain.Order, error) { return nil, nil }

func (r *fakeOrderRepo) UpdateStatus(id string, status string) error {
	r.updateOrderID = id
	r.updateStatus = status
	if order, ok := r.orders[id]; ok {
		order.Status = status
		order.UpdatedAt = time.Now()
		return nil
	}
	return sql.ErrNoRows
}

func (r *fakeOrderRepo) SubscribeToOrderUpdates(orderID string) <-chan *domain.Order {
	return make(chan *domain.Order)
}

type fakePaymentClient struct{}

func (fakePaymentClient) ProcessPayment(orderID string, amount int64, customerEmail string) (string, error) {
	return "Authorized", nil
}

type fakeOrderCache struct {
	values  map[string]*domain.Order
	deleted []string
	setTTL  time.Duration
}

func (c *fakeOrderCache) Get(id string) (*domain.Order, bool, error) {
	order, ok := c.values[id]
	if !ok {
		return nil, false, nil
	}
	copy := *order
	return &copy, true, nil
}

func (c *fakeOrderCache) Set(order *domain.Order, ttl time.Duration) error {
	copy := *order
	c.values[order.ID] = &copy
	c.setTTL = ttl
	return nil
}

func (c *fakeOrderCache) Delete(id string) error {
	c.deleted = append(c.deleted, id)
	delete(c.values, id)
	return nil
}

func TestGetOrderUsesCacheBeforeDatabase(t *testing.T) {
	repo := &fakeOrderRepo{orders: map[string]*domain.Order{}}
	cache := &fakeOrderCache{values: map[string]*domain.Order{
		"ORD-1": {ID: "ORD-1", Status: "Paid"},
	}}
	uc := NewOrderUseCase(repo, fakePaymentClient{})
	uc.SetCache(cache, time.Minute)

	order, err := uc.GetOrder("ORD-1")
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if order.Status != "Paid" {
		t.Fatalf("expected cached order status Paid, got %s", order.Status)
	}
	if repo.getCalls != 0 {
		t.Fatalf("expected database not to be called on cache hit, got %d calls", repo.getCalls)
	}
}

func TestGetOrderStoresDatabaseResultInCache(t *testing.T) {
	repo := &fakeOrderRepo{orders: map[string]*domain.Order{
		"ORD-2": {ID: "ORD-2", Status: "Pending"},
	}}
	cache := &fakeOrderCache{values: map[string]*domain.Order{}}
	uc := NewOrderUseCase(repo, fakePaymentClient{})
	uc.SetCache(cache, 5*time.Minute)

	order, err := uc.GetOrder("ORD-2")
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if order.Status != "Pending" {
		t.Fatalf("expected db order, got %s", order.Status)
	}
	if cache.values["ORD-2"] == nil {
		t.Fatalf("expected order to be cached after db read")
	}
	if cache.setTTL != 5*time.Minute {
		t.Fatalf("expected cache ttl 5m, got %s", cache.setTTL)
	}
}

func TestCancelOrderInvalidatesCacheAfterStatusUpdate(t *testing.T) {
	repo := &fakeOrderRepo{orders: map[string]*domain.Order{
		"ORD-3": {ID: "ORD-3", Status: "Pending"},
	}}
	cache := &fakeOrderCache{values: map[string]*domain.Order{
		"ORD-3": {ID: "ORD-3", Status: "Pending"},
	}}
	uc := NewOrderUseCase(repo, fakePaymentClient{})
	uc.SetCache(cache, time.Minute)

	if err := uc.CancelOrder("ORD-3"); err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if repo.updateStatus != "Cancelled" {
		t.Fatalf("expected cancelled status update, got %s", repo.updateStatus)
	}
	if len(cache.deleted) != 1 || cache.deleted[0] != "ORD-3" {
		t.Fatalf("expected cache invalidation for ORD-3, got %#v", cache.deleted)
	}
}
