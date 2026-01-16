CREATE TABLE IF NOT EXISTS payunit_transactions_init(
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    payunit_t_id      VARCHAR(100),                   -- “t_id” returned by PayUnit in response data
    payunit_t_sum     VARCHAR(100),                   -- “t_sum” returned
    payunit_t_url     TEXT,                           -- “t_url” returned

    transaction_id    VARCHAR(100) NOT NULL UNIQUE,  -- your internal transaction_id passed to PayUnit
    transaction_url TEXT,                     -- “transaction_url” returned 
    providers_json    JSONB                          -- full list of providers returned (as JSON array)  
);

CREATE TABLE IF NOT EXISTS payunit_payments (
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    transaction_id    VARCHAR(100) NOT NULL,       -- same as above / or a new one if you generate
    amount            BIGINT NOT NULL,
    payunit_id        VARCHAR(100),                -- the “id” returned by PayUnit in the data
    payment_status VARCHAR(50),            -- “PENDING”, “SUCCESS”, etc
    provider_transaction_id VARCHAR(100)       -- “provider_transaction_id” returned
);

CREATE TABLE IF NOT EXISTS payunit_payment_status (
    id                BIGSERIAL PRIMARY KEY,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),

    transaction_id    VARCHAR(100) NOT NULL,       -- your internal or the one you passed
    transaction_amount BIGINT,
    transaction_status VARCHAR(50),                -- e.g., “PENDING”, “FAILED”, “SUCCESS”, “CANCELLED” :contentReference[oaicite:1]{index=1}
    purchase_ref       VARCHAR(100),
    notify_url         TEXT,
    callback_url       TEXT,
    transaction_currency VARCHAR(10),
    transaction_gateway VARCHAR(50),
    pps_message            TEXT
);



ALTER TABLE IF EXISTS bookings
ADD COLUMN IF NOT EXISTS payunit_transactions_init_id BIGINT REFERENCES payunit_transactions_init(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS amount_to_pay BIGINT, 
ADD COLUMN IF NOT EXISTS transaction_id VARCHAR(150),
ADD COLUMN IF NOT EXISTS payunit_payment_id BIGINT REFERENCES payunit_payments(id) ON DELETE SET NULL; 

ALTER TABLE IF EXISTS payunit_payments
ADD COLUMN IF NOT EXISTS payunit_payment_status_id BIGINT REFERENCES payunit_payment_status(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_payunit_transactions_init_transaction_id ON payunit_transactions_init(transaction_id);
CREATE INDEX IF NOT EXISTS idx_payunit_payments_transaction_id ON payunit_payments(transaction_id);
CREATE INDEX IF NOT EXISTS idx_payunit_payment_status_transaction_id ON payunit_payment_status(transaction_id);