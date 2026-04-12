package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"payment-service/internal/repository"
	grpc_transport "payment-service/internal/transport/grpc"
	payhttp "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func main() {
	dbURL := getEnv("PAYMENT_DB_URL", "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable")
	grpcPort := getEnv("PAYMENT_GRPC_PORT", "50051")

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

	// Start gRPC server
	go startGRPCServer(grpcPort, uc)

	// Start HTTP server
	startHTTPServer(uc)
}

func startGRPCServer(port string, uc *usecase.PaymentUseCase) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_transport.LoggingInterceptor),
	)

	grpc_transport.RegisterPaymentServer(grpcServer, uc)

	log.Printf("Payment gRPC Server running on :%s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}
}

func startHTTPServer(uc *usecase.PaymentUseCase) {
	handler := payhttp.NewHandler(uc)

	r := gin.Default()

	r.POST("/payments", handler.CreatePayment)
	r.GET("/payments", handler.ListPayments)
	r.GET("/payments/:order_id", handler.GetPayment)

	httpPort := getEnv("PAYMENT_HTTP_PORT", "8081")
	log.Printf("Payment REST API running on :%s", httpPort)
	
	// Graceful shutdown
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%s", httpPort), r); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
