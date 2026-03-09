CREATE TABLE IF NOT EXISTS notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    TEXT NOT NULL,
    agent_id   UUID,
    type       TEXT NOT NULL,
    title      TEXT NOT NULL,
    message    TEXT NOT NULL DEFAULT '',
    metadata   JSONB DEFAULT '{}',
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_unread ON notifications (user_id, read, created_at DESC)
    WHERE read = FALSE;
CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);
