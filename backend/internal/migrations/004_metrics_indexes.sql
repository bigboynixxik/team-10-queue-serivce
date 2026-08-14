-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_queue_memberships_product_status ON queue_memberships (product_id, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_queue_memberships_product_status;
-- +goose StatementEnd