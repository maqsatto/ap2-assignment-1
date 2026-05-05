package usecase

import (
	"context"
	"fmt"
	"time"

	paymentv1 "github.com/maqsatto/ap2-generated-proto/gen/go/payment/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PaymentGRPCClient implements PaymentClient interface using gRPC
type PaymentGRPCClient struct {
	conn   *grpc.ClientConn
	client paymentv1.PaymentServiceClient
}

// NewPaymentGRPCClient creates a new gRPC payment client
func NewPaymentGRPCClient(grpcAddr string) (*PaymentGRPCClient, error) {
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to payment gRPC server: %w", err)
	}

	return &PaymentGRPCClient{
		conn:   conn,
		client: paymentv1.NewPaymentServiceClient(conn),
	}, nil
}

// ProcessPayment processes a payment via gRPC
func (c *PaymentGRPCClient) ProcessPayment(orderID string, amount int64, customerEmail string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "customer-email", customerEmail)

	req := &paymentv1.PaymentRequest{
		OrderId: orderID,
		Amount:  amount,
	}

	resp, err := c.client.ProcessPayment(ctx, req)
	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.Unavailable {
			return "", fmt.Errorf("payment service unavailable: %v", err)
		}
		return "", fmt.Errorf("failed to process payment: %v", err)
	}

	return resp.Status, nil
}

// Close closes the gRPC connection
func (c *PaymentGRPCClient) Close() error {
	return c.conn.Close()
}
