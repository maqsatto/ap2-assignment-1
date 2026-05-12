# Assignment 4 Architecture

```mermaid
flowchart LR
    Client[HTTP Client] -->|POST /orders| Order[Order Service]
    Client -->|GET /orders/:id| Order
    Client -->|rate limit counter| Redis[(Redis)]
    Order -->|gRPC ProcessPayment| Payment[Payment Service]
    Order --> OrderDB[(PostgreSQL order_db)]
    Order -->|cache-aside read/write| Redis
    Order -->|invalidate order:id after status update| Redis
    Payment --> PaymentDB[(PostgreSQL payment_db)]
    Payment -->|persistent payment.completed event| Broker[(RabbitMQ durable queue)]
    Broker -->|manual ACK| Notification[Notification Service]
    Broker -->|after 3 failed attempts| DLQ[(payment.completed.dlq)]
    Notification -->|check/mark notification:sent:payment_id| Redis
    Notification -->|EmailSender adapter| Provider[Simulated external provider]
    Provider -->|console log or transient failure| EmailLog[Notification result]
```

The Order Service uses Redis for cache-aside reads and the bonus API rate limiter. The Notification Service is a background worker: it consumes RabbitMQ jobs, uses Redis for idempotency, and calls an `EmailSender` adapter so provider logic is outside the worker flow.
