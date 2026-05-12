package consumer

import (
	"context"
	"encoding/json"
	"fmt"
)

type Processor struct {
	store  IdempotencyStore
	sender EmailSender
}

func NewProcessor(store IdempotencyStore, sender EmailSender) *Processor {
	return &Processor{store: store, sender: sender}
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
	processed, err := p.store.AlreadyProcessed(ctx, event.EventID)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	msg := EmailMessage{
		To:      event.CustomerEmail,
		OrderID: event.OrderID,
		Amount:  event.Amount,
		Status:  event.Status,
	}
	if err := p.sender.Send(ctx, msg); err != nil {
		return err
	}

	return p.store.MarkProcessed(ctx, event.EventID)
}
