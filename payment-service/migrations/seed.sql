INSERT INTO payments (id, order_id, transaction_id, amount, customer_email, status, created_at)
VALUES
    ('PAY-SEED-AUTH-001', 'ORD-SEED-PAID-001', 'TXN-SEED-0001', 99999, 'seed-paid@example.com', 'Authorized', NOW() - INTERVAL '2 hours'),
    ('PAY-SEED-DECLINED-001', 'ORD-SEED-FAILED-001', NULL, 150000, 'seed-failed@example.com', 'Declined', NOW() - INTERVAL '90 minutes')
ON CONFLICT (id) DO NOTHING;
