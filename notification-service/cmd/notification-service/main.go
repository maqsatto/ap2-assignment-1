package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"notification-service/internal/consumer"
)

func main() {
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	maxAttempts := getEnvInt32("NOTIFICATION_MAX_ATTEMPTS", 3)

	processor := consumer.NewProcessor(consumer.NewInMemoryIdempotencyStore(), os.Stdout)
	rabbitConsumer, err := connectRabbitConsumer(rabbitURL, processor, maxAttempts)
	if err != nil {
		log.Fatalf("failed to initialize rabbitmq consumer: %v", err)
	}
	defer rabbitConsumer.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Notification Service listening to queue %s", consumer.PaymentCompletedQueue)
	if err := rabbitConsumer.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("consumer stopped with error: %v", err)
	}
	log.Println("Notification Service shut down gracefully")
}

func connectRabbitConsumer(url string, processor *consumer.Processor, maxAttempts int32) (*consumer.RabbitConsumer, error) {
	var lastErr error
	for attempt := 1; attempt <= 60; attempt++ {
		rabbitConsumer, err := consumer.NewRabbitConsumer(url, processor, maxAttempts)
		if err == nil {
			log.Printf("Connected to RabbitMQ at %s", url)
			return rabbitConsumer, nil
		}
		lastErr = err
		log.Printf("RabbitMQ is not ready yet (attempt %d/60): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt32(key string, fallback int32) int32 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return int32(parsed)
}
