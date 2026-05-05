package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	PaymentCompletedQueue = "payment.completed"
	DeadLetterExchange    = "payment.dlx"
	DeadLetterQueue       = "payment.completed.dlq"
)

type RabbitPublisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbitPublisher(url string) (*RabbitPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	return &RabbitPublisher{conn: conn, ch: ch}, nil
}

func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(DeadLetterExchange, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}
	if _, err := ch.QueueDeclare(DeadLetterQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlq: %w", err)
	}
	if err := ch.QueueBind(DeadLetterQueue, DeadLetterQueue, DeadLetterExchange, false, nil); err != nil {
		return fmt.Errorf("bind dlq: %w", err)
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    DeadLetterExchange,
		"x-dead-letter-routing-key": DeadLetterQueue,
	}
	if _, err := ch.QueueDeclare(PaymentCompletedQueue, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare payment queue: %w", err)
	}
	return nil
}

func (p *RabbitPublisher) PublishPaymentCompleted(ctx context.Context, event PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payment completed event: %w", err)
	}

	confirms := p.ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	if err := p.ch.PublishWithContext(
		ctx,
		"",
		PaymentCompletedQueue,
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			MessageId:    event.EventID,
			Body:         body,
			Headers:      amqp.Table{"x-attempts": int32(1)},
		},
	); err != nil {
		return fmt.Errorf("publish payment completed event: %w", err)
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("broker negatively acknowledged payment event %s", event.EventID)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for broker confirm: %w", ctx.Err())
	}
}

func (p *RabbitPublisher) Close() error {
	var err error
	if p.ch != nil {
		if closeErr := p.ch.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if p.conn != nil {
		if closeErr := p.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if err != nil {
		log.Printf("rabbitmq close error: %v", err)
	}
	return err
}

type NoopPublisher struct{}

func (NoopPublisher) PublishPaymentCompleted(context.Context, PaymentCompletedEvent) error {
	return nil
}
