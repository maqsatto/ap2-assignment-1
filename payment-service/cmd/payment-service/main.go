package main

import (
	"database/sql"
	"log"
	"os"

	"payment-service/internal/repository"
	payhttp "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := getEnv("PAYMENT_DB_URL", "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo := repository.NewPaymentRepo(db)
	uc := usecase.NewPaymentUseCase(repo)
	handler := payhttp.NewHandler(uc)

	r := gin.Default()

	r.POST("/payments", handler.CreatePayment)
	r.GET("/payments", handler.ListPayments)
	r.GET("/payments/:order_id", handler.GetPayment)

	log.Printf("Payment Service running on :8081 (db=%s)", dbURL)
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
