INSERT INTO payments (id, order_id, transaction_id, amount, status, created_at)
VALUES
    ('PAY-SEED-AUTH-001', 'ORD-SEED-PAID-001', 'TXN-SEED-0001', 99999, 'Authorized', NOW() - INTERVAL '2 hours'),
    ('PAY-SEED-DECLINED-001', 'ORD-SEED-FAILED-001', NULL, 150000, 'Declined', NOW() - INTERVAL '90 minutes')
ON CONFLICT (id) DO NOTHING;