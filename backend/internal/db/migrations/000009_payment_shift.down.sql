ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_shift;

DROP TABLE IF EXISTS cash_movements;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS refund_items;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS shifts;
DROP TABLE IF EXISTS payment_methods;
