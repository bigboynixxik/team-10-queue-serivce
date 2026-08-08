-- +goose Up
-- +goose StatementBegin

CREATE INDEX idx_queue_memberships_user ON queue_memberships (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_queue_memberships_user;

-- +goose StatementEnd
