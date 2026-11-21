-- Deadline reminders from /dd. reminded_ms is set once the day-before reminder
-- has gone out; the row stays so /dd list can still show it until it is
-- deleted, and so a restart never sends the reminder twice.
CREATE TABLE deadlines (
    tenant_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    id TEXT NOT NULL,
    source_message_id TEXT NOT NULL,
    name TEXT NOT NULL,
    deadline_ms INTEGER NOT NULL,
    reminded_ms INTEGER,
    created_at_ms INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, account_id, chat_id, source_message_id)
);
CREATE INDEX deadlines_chat ON deadlines (tenant_id, account_id, chat_id);
