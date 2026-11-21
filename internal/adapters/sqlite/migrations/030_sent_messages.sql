-- Bot-sent messages, recorded at the single send boundary so /del can revoke
-- them for everyone. Rows are removed once revoked or forgotten.
CREATE TABLE sent_messages (
    tenant_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    provider_message_id TEXT NOT NULL,
    sent_at_ms INTEGER NOT NULL,
    PRIMARY KEY (tenant_id, account_id, chat_id, provider_message_id)
);
