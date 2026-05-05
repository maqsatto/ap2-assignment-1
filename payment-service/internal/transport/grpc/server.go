package grpc

import (
	"context"
	paymentv1 "github.com/maqsatto/ap2-generated-proto/gen/go/payment/v1"
	"payment-service/internal/domain"
	"payment-service/internal/usecase"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PaymentServer implements the gRPC PaymentService
type PaymentServer struct {
	uc *usecase.PaymentUseCase
	paymentv1.UnimplementedPaymentServiceServer
}

// NewPaymentServer creates a new gRPC payment server
func NewPaymentServer(uc *usecase.PaymentUseCase) *PaymentServer {
	return &PaymentServer{
		uc: uc,
	}
}

// ProcessPayment handles gRPC payment processing requests
func (s *PaymentServer) ProcessPayment(ctx context.Context, req *paymentv1.PaymentRequest) (*paymentv1.PaymentResponse, error) {
	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be greater than 0")
	}

	customerEmail := "unknown@example.com"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("customer-email"); len(values) > 0 && values[0] != "" {
			customerEmail = values[0]
		}
	}

	payment, err := s.uc.ProcessPayment(ctx, req.OrderId, req.Amount, customerEmail)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process payment: %v", err)
	}

	return &paymentv1.PaymentResponse{
		PaymentId:   payment.ID,
		OrderId:     payment.OrderID,
		Status:      payment.Status,
		Amount:      payment.Amount,
		ProcessedAt: timestamppb.New(payment.CreatedAt),
	}, nil
}

// RegisterPaymentServer registers the gRPC payment server with the given server
func RegisterPaymentServer(grpcServer *grpc.Server, uc *usecase.PaymentUseCase) {
	s := NewPaymentServer(uc)
	paymentv1.RegisterPaymentServiceServer(grpcServer, s)
}

// toDomain converts gRPC PaymentRequest to domain Payment
func toDomain(req *paymentv1.PaymentRequest) *domain.Payment {
	return &domain.Payment{
		OrderID: req.OrderId,
		Amount:  req.Amount,
	}
}
