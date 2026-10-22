-- +goose Up
-- Order within a message as the user attached it; created_at reflects upload
-- completion, which puts a small image ahead of a large one picked first.
ALTER TABLE attachments ADD COLUMN position integer;

-- +goose Down
ALTER TABLE attachments DROP COLUMN position;
