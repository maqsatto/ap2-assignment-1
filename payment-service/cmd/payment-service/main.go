package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"payment-service/internal/messaging"
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
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo, err := repository.NewPaymentRepo(db)
	if err != nil {
		log.Fatalf("failed to initialize payment repository: %v", err)
	}

	publisher, err := connectRabbitPublisher(rabbitURL)
	if err != nil {
		log.Fatalf("failed to initialize rabbitmq publisher: %v", err)
	}
	defer publisher.Close()

	uc := usecase.NewPaymentUseCase(repo, publisher)

	grpcServer, grpcListener := newGRPCServer(grpcPort, uc)
	go func() {
		log.Printf("Payment gRPC Server running on :%s", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("payment gRPC server stopped: %v", err)
		}
	}()

	httpServer := newHTTPServer(uc)
	go func() {
		log.Printf("Payment REST API running on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down Payment Service...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("payment HTTP shutdown error: %v", err)
	}
	grpcServer.GracefulStop()
}

func connectRabbitPublisher(url string) (*messaging.RabbitPublisher, error) {
	var lastErr error
	for attempt := 1; attempt <= 60; attempt++ {
		publisher, err := messaging.NewRabbitPublisher(url)
		if err == nil {
			log.Printf("Connected to RabbitMQ at %s", url)
			return publisher, nil
		}
		lastErr = err
		log.Printf("RabbitMQ is not ready yet (attempt %d/60): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func newGRPCServer(port string, uc *usecase.PaymentUseCase) (*grpc.Server, net.Listener) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_transport.LoggingInterceptor),
	)

	grpc_transport.RegisterPaymentServer(grpcServer, uc)

	return grpcServer, lis
}

func newHTTPServer(uc *usecase.PaymentUseCase) *http.Server {
	handler := payhttp.NewHandler(uc)

	r := gin.Default()

	r.POST("/payments", handler.CreatePayment)
	r.GET("/payments", handler.ListPayments)
	r.GET("/payments/:order_id", handler.GetPayment)

	httpPort := getEnv("PAYMENT_HTTP_PORT", "8081")
	return &http.Server{Addr: fmt.Sprintf(":%s", httpPort), Handler: r}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
