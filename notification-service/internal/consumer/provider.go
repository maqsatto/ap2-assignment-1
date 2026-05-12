package consumer

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"
)

type EmailMessage struct {
	To      string
	OrderID string
	Amount  int64
	Status  string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type SimulatedEmailSender struct {
	output      io.Writer
	latency     time.Duration
	failureRate int
	rand        *rand.Rand
}

func NewSimulatedEmailSender(output io.Writer, latency time.Duration, failureRate int) *SimulatedEmailSender {
	if latency <= 0 {
		latency = 300 * time.Millisecond
	}
	if failureRate < 0 {
		failureRate = 0
	}
	if failureRate > 100 {
		failureRate = 100
	}
	return &SimulatedEmailSender{
		output:      output,
		latency:     latency,
		failureRate: failureRate,
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *SimulatedEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.latency):
	}

	if msg.To == "fail@example.com" {
		return fmt.Errorf("simulated permanent notification failure for %s", msg.To)
	}
	if s.failureRate > 0 && s.rand.Intn(100) < s.failureRate {
		return fmt.Errorf("simulated temporary provider failure")
	}

	if _, err := fmt.Fprintf(
		s.output,
		"[Notification] Sent email to %s for Order #%s. Amount: $%.2f\n",
		msg.To,
		msg.OrderID,
		float64(msg.Amount)/100,
	); err != nil {
		return fmt.Errorf("write notification log: %w", err)
	}
	return nil
}
