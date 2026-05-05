INSERT INTO orders (id, customer_id, customer_email, item_name, amount, status, created_at, updated_at)
VALUES
    ('ORD-SEED-PENDING-001', 'CUST-SEED-001', 'seed-pending@example.com', 'Seed Pending Item', 45000, 'Pending', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours'),
    ('ORD-SEED-PAID-001', 'CUST-SEED-002', 'seed-paid@example.com', 'Seed Paid Item', 99999, 'Paid', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours'),
    ('ORD-SEED-FAILED-001', 'CUST-SEED-003', 'seed-failed@example.com', 'Seed Failed Item', 150000, 'Failed', NOW() - INTERVAL '90 minutes', NOW() - INTERVAL '90 minutes'),
    ('ORD-SEED-CANCELLED-001', 'CUST-SEED-004', 'seed-cancelled@example.com', 'Seed Cancelled Item', 25000, 'Cancelled', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour')
ON CONFLICT (id) DO NOTHING;
