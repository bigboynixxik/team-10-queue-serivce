-- +goose Up
-- +goose StatementBegin
CREATE TABLE stock_decrement_outbox (
    id UUID PRIMARY KEY,
    right_token TEXT NOT NULL UNIQUE REFERENCES rights (token),
    order_id TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stock_decrement_outbox_due
    ON stock_decrement_outbox (next_attempt_at)
    WHERE delivered_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS stock_decrement_outbox;
-- +goose StatementEnd
