-- +migrate Up
CREATE TABLE IF NOT EXISTS orders (
                                      number      TEXT PRIMARY KEY,
                                      user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'NEW',
    accrual     NUMERIC(20, 2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

CREATE INDEX IF NOT EXISTS orders_user_id_idx ON orders(user_id);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders(status);
CREATE INDEX IF NOT EXISTS orders_pending_idx ON orders(uploaded_at)
    WHERE status IN ('NEW', 'PROCESSING');

-- +migrate Down
DROP INDEX IF EXISTS orders_pending_idx;
DROP INDEX IF EXISTS orders_status_idx;
DROP INDEX IF EXISTS orders_user_id_idx;
DROP TABLE IF EXISTS orders;