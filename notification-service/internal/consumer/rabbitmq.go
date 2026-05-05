package consumer

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	PaymentCompletedQueue = "payment.completed"
	DeadLetterExchange    = "payment.dlx"
	DeadLetterQueue       = "payment.completed.dlq"
	AttemptHeader         = "x-attempts"
)

type FailureAction string

const (
	FailureActionRetry      FailureAction = "retry"
	FailureActionDeadLetter FailureAction = "dead-letter"
)

type FailureRoute struct {
	Action      FailureAction
	NextAttempt int32
}

func DecideFailureRoute(headers amqp.Table, maxAttempts int32) FailureRoute {
	attempt := headerInt32(headers[AttemptHeader])
	if attempt <= 0 {
		attempt = 1
	}
	if attempt >= maxAttempts {
		return FailureRoute{Action: FailureActionDeadLetter, NextAttempt: attempt}
	}
	return FailureRoute{Action: FailureActionRetry, NextAttempt: attempt + 1}
}

func headerInt32(v any) int32 {
	switch value := v.(type) {
	case int32:
		return value
	case int64:
		return int32(value)
	case int:
		return int32(value)
	case int16:
		return int32(value)
	case byte:
		return int32(value)
	default:
		return 0
	}
}

type RabbitConsumer struct {
	conn         *amqp.Connection
	ch           *amqp.Channel
	processor    *Processor
	maxAttempts  int32
	pollingQueue string
}

func NewRabbitConsumer(url string, processor *Processor, maxAttempts int32) (*RabbitConsumer, error) {
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

	if err := ch.Qos(1, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("configure qos: %w", err)
	}

	return &RabbitConsumer{
		conn:         conn,
		ch:           ch,
		processor:    processor,
		maxAttempts:  maxAttempts,
		pollingQueue: PaymentCompletedQueue,
	}, nil
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

func (c *RabbitConsumer) Run(ctx context.Context) error {
	deliveries, err := c.ch.Consume(c.pollingQueue, "notification-service", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-deliveries:
			if !ok {
				return nil
			}
			c.handleDelivery(ctx, msg)
		}
	}
}

func (c *RabbitConsumer) handleDelivery(ctx context.Context, msg amqp.Delivery) {
	if err := c.processor.Process(ctx, msg.Body); err == nil {
		if ackErr := msg.Ack(false); ackErr != nil {
			log.Printf("failed to ack message %s: %v", msg.MessageId, ackErr)
		}
		return
	} else {
		log.Printf("notification processing failed for message %s: %v", msg.MessageId, err)
	}

	route := DecideFailureRoute(msg.Headers, c.maxAttempts)
	switch route.Action {
	case FailureActionRetry:
		if err := c.retry(ctx, msg, route.NextAttempt); err != nil {
			log.Printf("failed to republish retry for message %s: %v", msg.MessageId, err)
			if nackErr := msg.Nack(false, true); nackErr != nil {
				log.Printf("failed to nack message %s: %v", msg.MessageId, nackErr)
			}
			return
		}
		if ackErr := msg.Ack(false); ackErr != nil {
			log.Printf("failed to ack original retry message %s: %v", msg.MessageId, ackErr)
		}
	case FailureActionDeadLetter:
		log.Printf("moving message %s to DLQ after %d attempts", msg.MessageId, route.NextAttempt)
		if nackErr := msg.Nack(false, false); nackErr != nil {
			log.Printf("failed to dead-letter message %s: %v", msg.MessageId, nackErr)
		}
	}
}

func (c *RabbitConsumer) retry(ctx context.Context, msg amqp.Delivery, nextAttempt int32) error {
	headers := amqp.Table{}
	for key, value := range msg.Headers {
		headers[key] = value
	}
	headers[AttemptHeader] = nextAttempt

	return c.ch.PublishWithContext(ctx, "", c.pollingQueue, true, false, amqp.Publishing{
		ContentType:   msg.ContentType,
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now(),
		MessageId:     msg.MessageId,
		CorrelationId: msg.CorrelationId,
		Headers:       headers,
		Body:          msg.Body,
	})
}

func (c *RabbitConsumer) Close() error {
	var err error
	if c.ch != nil {
		if closeErr := c.ch.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
