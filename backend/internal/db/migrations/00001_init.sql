-- +goose Up
CREATE TABLE conversations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX conversations_updated_at_idx ON conversations (updated_at DESC);

CREATE TABLE messages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user', 'assistant')),
    content         text NOT NULL DEFAULT '',
    status          text NOT NULL DEFAULT 'complete' CHECK (status IN ('complete', 'stopped', 'error')),
    model           text NOT NULL DEFAULT '',
    -- clock_timestamp, not now(): two inserts in one transaction must still sort apart.
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX messages_conversation_created_idx ON messages (conversation_id, created_at);

CREATE TABLE attachments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- NULL until the message that uses it is sent; orphans are cleaned up later.
    message_id  uuid REFERENCES messages (id) ON DELETE CASCADE,
    mime        text NOT NULL CHECK (mime IN ('image/jpeg', 'image/png', 'image/webp')),
    size_bytes  integer NOT NULL CHECK (size_bytes > 0),
    width       integer NOT NULL CHECK (width > 0),
    height      integer NOT NULL CHECK (height > 0),
    sha256      text NOT NULL,
    storage_key text NOT NULL UNIQUE,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX attachments_message_idx ON attachments (message_id);
CREATE INDEX attachments_orphan_idx ON attachments (created_at) WHERE message_id IS NULL;

-- +goose Down
DROP TABLE attachments;
DROP TABLE messages;
DROP TABLE conversations;
