package http

import (
	"net/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.PaymentUseCase
}

func NewHandler(uc *usecase.PaymentUseCase) *Handler {
	return &Handler{uc: uc}
}

type CreatePaymentRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	Amount  int64  `json:"amount" binding:"required"`
}

func (h *Handler) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.uc.ProcessPayment(req.OrderID, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, payment)
}

func (h *Handler) GetPayment(c *gin.Context) {
	orderID := c.Param("order_id")

	payment, err := h.uc.GetPaymentByOrderID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	c.JSON(http.StatusOK, payment)
}

func (h *Handler) ListPayments(c *gin.Context) {
	payments, err := h.uc.ListPayments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}
