CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL UNIQUE,
    transaction_id VARCHAR(255),
    amount BIGINT NOT NULL,
    customer_email VARCHAR(255) NOT NULL DEFAULT 'unknown@example.com',
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order_id ON payments(order_id);
