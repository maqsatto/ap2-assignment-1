# Advanced Programming 2 - Assignment 3

Event-Driven Architecture with RabbitMQ. This repository continues the Assignment 2 gRPC solution and adds asynchronous notifications:

```text
Order Service -> gRPC -> Payment Service -> RabbitMQ -> Notification Service
```

## What Changed From Assignment 2

- Order Service still calls Payment Service synchronously through gRPC.
- Payment Service is now also a RabbitMQ producer.
- Notification Service is a new independent consumer service.
- Email sending is simulated by logs, so Payment no longer calls Notification directly.
- Docker Compose now runs Order, Payment, Notification, PostgreSQL databases, and RabbitMQ.

## Architecture Diagram

```mermaid
flowchart LR
    Client[HTTP Client] -->|POST /orders| Order[Order Service]
    Order -->|ProcessPayment gRPC| Payment[Payment Service]
    Order --> OrderDB[(order-db)]
    Payment --> PaymentDB[(payment-db)]
    Payment -->|durable JSON payment.completed| Rabbit[(RabbitMQ)]
    Rabbit -->|manual ACK consumer| Notification[Notification Service]
    Rabbit --> DLQ[(payment.completed.dlq)]

    Notification -->|console log| Log[Notification log]
```

## Services

### Order Service

- Exposes REST API on `http://localhost:18080`.
- Creates orders and calls Payment Service via gRPC.
- Accepts `customer_email` and forwards it to Payment Service through gRPC metadata.
- Has graceful shutdown for HTTP and gRPC servers.

Example:

```bash
curl -X POST http://localhost:18080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: demo-1" \
  -d '{"customer_id":"cust-123","customer_email":"user@example.com","item_name":"keyboard","amount":9999}'
```

### Payment Service

- Exposes REST API on `http://localhost:18081`.
- Exposes gRPC API on `localhost:50051`.
- Stores payment records in PostgreSQL.
- After the payment record is saved, publishes a persistent JSON event to RabbitMQ queue `payment.completed`.
- Uses RabbitMQ publisher confirms so the service knows the broker accepted the event.

Event payload:

```json
{
  "event_id": "PAY-...",
  "order_id": "ORD-...",
  "amount": 9999,
  "customer_email": "user@example.com",
  "status": "Authorized"
}
```

### Notification Service

- Consumes only RabbitMQ messages.
- Does not import or call Order Service or Payment Service.
- Logs the simulated email:

```text
[Notification] Sent email to user@example.com for Order #ORD-123. Amount: $99.99
```

## Reliability

### Durable Queues and Persistent Messages

RabbitMQ queue `payment.completed` is declared as durable. Payment Service publishes messages with `DeliveryMode: Persistent`, so messages survive broker restarts when RabbitMQ persists them to disk.

### Manual ACK Logic

Notification Service consumes with `autoAck=false`.

- If the event is processed and the log is printed, the service calls `Ack(false)`.
- If processing fails before the log is printed, the message is not acknowledged as successful.
- This gives at-least-once delivery: if the consumer crashes before ACK, RabbitMQ can redeliver the message.

### Idempotency Strategy

Notification Service keeps an in-memory map of processed `event_id` values.

- First delivery: log is printed, then `event_id` is marked processed, then ACK is sent.
- Duplicate delivery: `event_id` is already known, so the service skips the log and ACKs the duplicate.

This prevents duplicate notification logs for the same event ID. For a production system, the same strategy would use a database table instead of memory.

### DLQ Bonus

The queue is configured with:

- dead-letter exchange: `payment.dlx`
- dead-letter queue: `payment.completed.dlq`
- max processing attempts: `3`

On failure, Notification Service republishes the message with an incremented `x-attempts` header. On the third failed attempt it rejects the message with `requeue=false`, and RabbitMQ moves it to `payment.completed.dlq`.

Permanent failure simulation:

- Create an order with `customer_email` equal to `fail@example.com`.
- Notification Service treats that address as a permanent notification provider error.
- After 3 attempts, the message is sent to the DLQ.

```bash
curl -X POST http://localhost:18080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-dlq","customer_email":"fail@example.com","item_name":"dlq-demo","amount":5000}'
```

Open RabbitMQ UI at `http://localhost:15672` with `guest` / `guest` and check `payment.completed.dlq`.

## Running

```bash
docker compose up --build
```

Services and ports:

| Component | URL |
| --- | --- |
| Order REST | `http://localhost:18080` |
| Payment REST | `http://localhost:18081` |
| Payment gRPC | `localhost:50051` |
| Order gRPC stream | `localhost:50052` |
| RabbitMQ AMQP | `localhost:5672` |
| RabbitMQ UI | `http://localhost:15672` |

If you previously ran Assignment 2 containers, reset volumes once so the updated schemas and RabbitMQ topology are created cleanly:

```bash
docker compose down -v
docker compose up --build
```

## Demo Flow

1. Start everything:

```bash
docker compose up --build
```

2. Create a successful order:

```bash
curl -X POST http://localhost:18080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-123","customer_email":"user@example.com","item_name":"mouse","amount":9999}'
```

3. Check Notification Service logs. You should see:

```text
[Notification] Sent email to user@example.com for Order #ORD-.... Amount: $99.99
```

4. Stop only Notification Service:

```bash
docker compose stop notification-service
```

5. Create another order. The event remains in RabbitMQ because the queue is durable and the consumer is offline.

6. Start Notification Service again:

```bash
docker compose start notification-service
```

7. The pending notification is consumed and ACKed after the log is printed.

8. Demonstrate DLQ with `fail@example.com` and inspect `payment.completed.dlq` in RabbitMQ UI.

## Configuration

| Variable | Service | Default |
| --- | --- | --- |
| `ORDER_DB_URL` | Order | `postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable` |
| `PAYMENT_GRPC_ADDR` | Order | `localhost:50051` |
| `ORDER_GRPC_PORT` | Order | `50052` |
| `PAYMENT_DB_URL` | Payment | `postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable` |
| `PAYMENT_GRPC_PORT` | Payment | `50051` |
| `PAYMENT_HTTP_PORT` | Payment | `8081` |
| `RABBITMQ_URL` | Payment, Notification | `amqp://guest:guest@localhost:5672/` |
| `NOTIFICATION_MAX_ATTEMPTS` | Notification | `3` |

## Source Layout

```text
order-service/          REST + gRPC client + order DB
payment-service/        gRPC/REST payment processor + RabbitMQ producer
notification-service/   RabbitMQ consumer + idempotency + ACK/DLQ handling
proto/                  Assignment 2 proto contracts
docker-compose.yml      Full runtime environment
openapi.yaml            REST API contract
```
