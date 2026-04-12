package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"order-service/internal/repository"
	ordhttp "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := getEnv("ORDER_DB_URL", "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable")
	paymentGRPCAddr := getEnv("PAYMENT_GRPC_ADDR", "localhost:50051")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo := repository.NewOrderRepo(db)
	
	// Create gRPC payment client
	paymentClient, err := usecase.NewPaymentGRPCClient(paymentGRPCAddr)
	if err != nil {
		log.Fatalf("failed to create payment gRPC client: %v", err)
	}
	defer paymentClient.Close()
	
	uc := usecase.NewOrderUseCase(repo, paymentClient)
	handler := ordhttp.NewHandler(uc)

	r := gin.Default()

	r.POST("/orders", handler.CreateOrder)
	r.GET("/orders", handler.ListOrders)
	r.GET("/orders/:id", handler.GetOrder)
	r.PATCH("/orders/:id/cancel", handler.CancelOrder)

	// Graceful shutdown
	go func() {
		log.Printf("Order Service running on :8080 (db=%s, payment_grpc=%s)", dbURL, paymentGRPCAddr)
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Order Service...")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
