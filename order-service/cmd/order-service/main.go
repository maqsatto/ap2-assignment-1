package main

import (
	"database/sql"
	"log"
	"os"

	"order-service/internal/repository"
	ordhttp "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := getEnv("ORDER_DB_URL", "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable")
	paymentBaseURL := getEnv("PAYMENT_BASE_URL", "http://localhost:8081")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo := repository.NewOrderRepo(db)
	paymentClient := usecase.NewPaymentHTTPClient(paymentBaseURL)
	uc := usecase.NewOrderUseCase(repo, paymentClient)
	handler := ordhttp.NewHandler(uc)

	r := gin.Default()

	r.POST("/orders", handler.CreateOrder)
	r.GET("/orders", handler.ListOrders)
	r.GET("/orders/:id", handler.GetOrder)
	r.PATCH("/orders/:id/cancel", handler.CancelOrder)

	log.Printf("Order Service running on :8080 (db=%s, payment=%s)", dbURL, paymentBaseURL)
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
