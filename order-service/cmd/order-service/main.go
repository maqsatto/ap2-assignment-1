package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"order-service/internal/repository"
	grpc_transport "order-service/internal/transport/grpc"
	ordhttp "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func main() {
	dbURL := getEnv("ORDER_DB_URL", "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable")
	paymentGRPCAddr := getEnv("PAYMENT_GRPC_ADDR", "localhost:50051")
	orderGRPCPort := getEnv("ORDER_GRPC_PORT", "50052")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo, err := repository.NewOrderRepo(db)
	if err != nil {
		log.Fatalf("failed to initialize order repository: %v", err)
	}
	
	// Create gRPC payment client
	paymentClient, err := usecase.NewPaymentGRPCClient(paymentGRPCAddr)
	if err != nil {
		log.Fatalf("failed to create payment gRPC client: %v", err)
	}
	defer paymentClient.Close()
	
	uc := usecase.NewOrderUseCase(repo, paymentClient)
	handler := ordhttp.NewHandler(uc)

	// Start gRPC server for order streaming
	go startGRPCServer(orderGRPCPort, repo)

	// Start HTTP server
	r := gin.Default()

	r.POST("/orders", handler.CreateOrder)
	r.GET("/orders", handler.ListOrders)
	r.GET("/orders/:id", handler.GetOrder)
	r.PATCH("/orders/:id/cancel", handler.CancelOrder)

	// Graceful shutdown
	go func() {
		log.Printf("Order Service running on :8080 (db=%s, payment_grpc=%s, order_grpc=%s)", dbURL, paymentGRPCAddr, orderGRPCPort)
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Order Service...")
}

func startGRPCServer(port string, repo *repository.OrderRepo) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()

	grpc_transport.RegisterOrderServer(grpcServer, repo)

	log.Printf("Order gRPC Server running on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
