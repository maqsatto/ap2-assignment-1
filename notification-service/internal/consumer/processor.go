package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type Processor struct {
	store  IdempotencyStore
	output io.Writer
}

func NewProcessor(store IdempotencyStore, output io.Writer) *Processor {
	return &Processor{store: store, output: output}
}

func (p *Processor) Process(ctx context.Context, body []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var event PaymentCompletedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode payment event: %w", err)
	}
	if event.EventID == "" {
		return fmt.Errorf("payment event has empty event_id")
	}
	if p.store.AlreadyProcessed(event.EventID) {
		return nil
	}

	if event.CustomerEmail == "fail@example.com" {
		return fmt.Errorf("simulated permanent notification failure for %s", event.CustomerEmail)
	}

	if _, err := fmt.Fprintf(
		p.output,
		"[Notification] Sent email to %s for Order #%s. Amount: $%.2f\n",
		event.CustomerEmail,
		event.OrderID,
		float64(event.Amount)/100,
	); err != nil {
		return fmt.Errorf("write notification log: %w", err)
	}

	p.store.MarkProcessed(event.EventID)
	return nil
}
