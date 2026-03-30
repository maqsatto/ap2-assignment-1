package http

import (
	"errors"
	"net/http"
	"order-service/internal/usecase"
	"sync"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc              *usecase.OrderUseCase
	idempotencyKeys map[string]bool
	mu              sync.Mutex
}

func NewHandler(uc *usecase.OrderUseCase) *Handler {
	return &Handler{
		uc:              uc,
		idempotencyKeys: make(map[string]bool),
	}
}

type CreateOrderRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	ItemName   string `json:"item_name" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
}

func (h *Handler) CreateOrder(c *gin.Context) {
	idempotencyKey := c.GetHeader("Idempotency-Key")

	if idempotencyKey != "" {
		h.mu.Lock()
		if h.idempotencyKeys[idempotencyKey] {
			h.mu.Unlock()
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate request: idempotency key already used"})
			return
		}
		h.idempotencyKeys[idempotencyKey] = true
		h.mu.Unlock()
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.uc.CreateOrder(req.CustomerID, req.ItemName, req.Amount)
	if err != nil {
		if errors.Is(err, usecase.ErrPaymentServiceUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (h *Handler) GetOrder(c *gin.Context) {
	id := c.Param("id")

	order, err := h.uc.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *Handler) ListOrders(c *gin.Context) {
	orders, err := h.uc.ListOrders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	id := c.Param("id")

	if err := h.uc.CancelOrder(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order cancelled"})
}
