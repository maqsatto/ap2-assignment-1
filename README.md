# Advanced Programming 2 - Assignment 4

Performance optimization and reliable background jobs. This repository continues the previous microservices and adds Redis caching, Redis idempotency, exponential backoff, and a provider adapter for notifications.

```text
Client -> Order Service -> gRPC -> Payment Service -> RabbitMQ -> Notification Worker -> Provider Adapter
              |                                      |
              +-------------- Redis -----------------+
```

## What Changed From Assignment 3

- Order Service uses Redis cache-aside for `GET /orders/:id`.
- Order Service invalidates the order cache right after status changes.
- Order Service has a Redis-backed API rate limiter for the bonus task.
- Notification Service now uses a provider adapter (`EmailSender`) instead of writing directly inside business logic.
- Notification idempotency moved from memory to Redis, so retries do not send duplicate emails after restarts.
- Failed notification jobs use exponential backoff before retrying.
- Docker Compose now runs Redis together with PostgreSQL and RabbitMQ.

## Architecture Diagram

```mermaid
flowchart LR
    Client[HTTP Client] -->|POST /orders| Order[Order Service]
    Client -->|GET /orders/:id| Order
    Client -->|rate limit by IP| Redis[(Redis)]
    Order -->|ProcessPayment gRPC| Payment[Payment Service]
    Order --> OrderDB[(order-db)]
    Order -->|cache-aside order:id TTL 5m| Redis
    Order -->|delete order:id after status update| Redis
    Payment --> PaymentDB[(payment-db)]
    Payment -->|durable JSON payment.completed| Rabbit[(RabbitMQ)]
    Rabbit -->|manual ACK consumer| Notification[Notification Service]
    Rabbit --> DLQ[(payment.completed.dlq)]
    Notification -->|idempotency key notification:sent:payment_id| Redis
    Notification -->|EmailSender adapter| Provider[Simulated provider]
    Provider -->|console log or failure| Log[Notification log]
```

## Services

### Order Service

- Exposes REST API on `http://localhost:18080`.
- Creates orders and calls Payment Service via gRPC.
- Accepts `customer_email` and forwards it to Payment Service through gRPC metadata.
- Uses Redis for cache-aside reads on `GET /orders/:id`.
- Deletes the cached `order:{id}` key after status updates such as `Paid`, `Failed`, or `Cancelled`.
- Limits clients to `RATE_LIMIT_REQUESTS` per `RATE_LIMIT_WINDOW` using Redis counters.
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
- Uses `EmailSender` interface, selected with `PROVIDER_MODE`.
- The included `SIMULATED` provider sleeps for `SIMULATED_PROVIDER_LATENCY` and randomly fails according to `SIMULATED_PROVIDER_FAILURE_RATE`.
- Stores processed payment IDs in Redis with `NOTIFICATION_IDEMPOTENCY_TTL`.
- Logs the simulated email after the provider succeeds:

```text
[Notification] Sent email to user@example.com for Order #ORD-123. Amount: $99.99
```

## Reliability

### Redis Cache-Aside

`GET /orders/:id` first checks Redis using key `order:{id}`. On a cache miss, the service reads PostgreSQL and stores the order in Redis for `ORDER_CACHE_TTL`, which is `5m` by default.

Invalidation happens immediately after database status updates. For example, after payment authorization the database row is changed to `Paid`, then `order:{id}` is deleted. The next read goes to PostgreSQL and refreshes Redis with the newest status.

### API Rate Limiter Bonus

The Order Service uses Redis key `rate_limit:{client_ip}` and increments it for each request. The key expires after `RATE_LIMIT_WINDOW`. When the value is higher than `RATE_LIMIT_REQUESTS`, the API returns HTTP `429 Too Many Requests`.

### Durable Queues and Persistent Messages

RabbitMQ queue `payment.completed` is declared as durable. Payment Service publishes messages with `DeliveryMode: Persistent`, so messages survive broker restarts when RabbitMQ persists them to disk.

### Manual ACK Logic

Notification Service consumes with `autoAck=false`.

- If the event is processed and the log is printed, the service calls `Ack(false)`.
- If processing fails before the log is printed, the message is not acknowledged as successful.
- This gives at-least-once delivery: if the consumer crashes before ACK, RabbitMQ can redeliver the message.

### Idempotency Strategy

Notification Service stores processed payment event IDs in Redis.

- First delivery: Redis key `notification:sent:{payment_id}` is missing, provider is called, then the key is saved as `sent`.
- Duplicate delivery: Redis key already exists, so the worker skips the provider call and ACKs the duplicate.

This prevents duplicate notification logs for the same payment even if RabbitMQ redelivers a message or the worker restarts.

### Provider Adapter and Retry Logic

Notification worker depends on the `EmailSender` interface. The current adapter is simulated, which is allowed by the assignment. It behaves like an unstable external API by sleeping and sometimes returning an error.

If sending fails, the RabbitMQ message is not treated as successful. The worker republishes it with the next `x-attempts` header and waits with exponential backoff:

```text
attempt 1 -> 2s
attempt 2 -> 4s
attempt 3 -> 8s
```

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
| Redis | `localhost:6379` |

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

9. Demonstrate cache read:

```bash
curl http://localhost:18080/orders/ORD-your-id
curl http://localhost:18080/orders/ORD-your-id
```

The first request fills Redis. The second request is served from cache until TTL expires or the order status changes.

10. Demonstrate rate limit:

```bash
for i in {1..12}; do curl -i http://localhost:18080/orders; done
```

After the configured limit, the response becomes `429 Too Many Requests`.

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
| `REDIS_ADDR` | Order, Notification | `localhost:6379` |
| `ORDER_CACHE_TTL` | Order | `5m` |
| `RATE_LIMIT_REQUESTS` | Order | `10` |
| `RATE_LIMIT_WINDOW` | Order | `1m` |
| `PROVIDER_MODE` | Notification | `SIMULATED` |
| `SIMULATED_PROVIDER_LATENCY` | Notification | `300ms` |
| `SIMULATED_PROVIDER_FAILURE_RATE` | Notification | `15` |
| `NOTIFICATION_MAX_ATTEMPTS` | Notification | `3` |
| `NOTIFICATION_IDEMPOTENCY_TTL` | Notification | `24h` |

Configuration values are listed in `.env.example`; the local Docker setup uses `.env`.

## Source Layout

```text
order-service/          REST + gRPC client + order DB
payment-service/        gRPC/REST payment processor + RabbitMQ producer
notification-service/   RabbitMQ worker + Redis idempotency + provider adapter
proto/                  Assignment 2 proto contracts
docker-compose.yml      Full runtime environment
openapi.yaml            REST API contract
```
