-- +migrate Up
CREATE TABLE IF NOT EXISTS balances (
                                        user_id   BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current   NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (current >= 0),
    withdrawn NUMERIC(20, 2) NOT NULL DEFAULT 0 CHECK (withdrawn >= 0)
    );

-- +migrate Down
DROP TABLE IF EXISTS balances;