-- +goose Up
-- +goose StatementBegin
CREATE TYPE right_status AS ENUM ('ACTIVE', 'EXPIRED', 'USED');

CREATE TABLE rights (
                        token       TEXT PRIMARY KEY,
                        user_id     TEXT NOT NULL,
                        product_id  TEXT NOT NULL,
                        quantity    INTEGER NOT NULL CHECK (quantity > 0),
                        status      right_status NOT NULL DEFAULT 'ACTIVE',
                        order_id    TEXT,
                        created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
                        expires_at  TIMESTAMPTZ NOT NULL,
                        used_at     TIMESTAMPTZ
);

CREATE INDEX idx_rights_user_product ON rights (user_id, product_id);
CREATE INDEX idx_rights_product ON rights (product_id);

CREATE TYPE membership_status AS ENUM (
    'QUEUED', 'RIGHT_ACTIVE', 'OFFER_PENDING', 'DECLINED', 'PURCHASED', 'SOLD_OUT'
    );

CREATE TABLE queue_memberships (
                                   id                  BIGSERIAL PRIMARY KEY,
                                   product_id          TEXT NOT NULL,
                                   user_id             TEXT NOT NULL,
                                   status              membership_status NOT NULL,
                                   quantity            INTEGER NOT NULL CHECK (quantity > 0),
                                   available_quantity  INTEGER,
                                   current_token       TEXT REFERENCES rights (token),
                                   expires_at          TIMESTAMPTZ,
                                   created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
                                   updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
                                   UNIQUE (product_id, user_id)
);

CREATE TABLE product_stock (
                               product_id     TEXT PRIMARY KEY,
                               product_count  INTEGER NOT NULL CHECK (product_count >= 0),
                               total_stock    INTEGER NOT NULL,
                               updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS product_stock;
DROP TABLE IF EXISTS queue_memberships;
DROP TABLE IF EXISTS rights;

DROP TYPE IF EXISTS membership_status;
DROP TYPE IF EXISTS right_status;
-- +goose StatementEnd