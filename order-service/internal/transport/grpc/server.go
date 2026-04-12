package grpc

import (
	"log"
	"order-service/internal/domain"
	orderv1 "order-service/proto/gen/go/order/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OrderServer implements the gRPC OrderService with streaming
type OrderServer struct {
	repo domain.OrderRepository
	orderv1.UnimplementedOrderServiceServer
}

// NewOrderServer creates a new gRPC order server
func NewOrderServer(repo domain.OrderRepository) *OrderServer {
	return &OrderServer{
		repo: repo,
	}
}

// SubscribeToOrderUpdates streams real-time status updates for a specific order
func (s *OrderServer) SubscribeToOrderUpdates(req *orderv1.OrderRequest, stream orderv1.OrderService_SubscribeToOrderUpdatesServer) error {
	if req.OrderId == "" {
		return status.Errorf(codes.InvalidArgument, "order_id is required")
	}

	// Verify order exists
	_, err := s.repo.GetByID(req.OrderId)
	if err != nil {
		return status.Errorf(codes.NotFound, "order %s not found", req.OrderId)
	}

	// Subscribe to order updates from repository
	updates := s.repo.SubscribeToOrderUpdates(req.OrderId)

	// Stream updates to the client
	for order := range updates {
		update := &orderv1.OrderStatusUpdate{
			OrderId:   order.ID,
			Status:    order.Status,
			UpdatedAt: timestamppb.New(order.CreatedAt),
		}

		if err := stream.Send(update); err != nil {
			log.Printf("failed to send order update: %v", err)
			return status.Errorf(codes.Internal, "failed to send update: %v", err)
		}
	}

	return nil
}

// RegisterOrderServer registers the gRPC order server with the given server
func RegisterOrderServer(grpcServer *grpc.Server, repo domain.OrderRepository) {
	s := NewOrderServer(repo)
	orderv1.RegisterOrderServiceServer(grpcServer, s)
}
