-- Drop indexes
DROP INDEX IF EXISTS idx_payunit_payment_status_transaction_id;
DROP INDEX IF EXISTS idx_payunit_payments_transaction_id;
DROP INDEX IF EXISTS idx_payunit_transactions_init_transaction_id;

-- Drop foreign key columns from payunit_payments
ALTER TABLE IF EXISTS payunit_payments
DROP COLUMN IF EXISTS payunit_payment_status_id;

-- Drop foreign key columns from bookings
ALTER TABLE IF EXISTS bookings
DROP COLUMN IF EXISTS payunit_payment_id,
DROP COLUMN IF EXISTS amount_to_pay,
DROP COLUMN IF EXISTS payunit_transactions_init_id;

-- Drop tables
DROP TABLE IF EXISTS payunit_payment_status;
DROP TABLE IF EXISTS payunit_payments;
DROP TABLE IF EXISTS payunit_transactions_init;