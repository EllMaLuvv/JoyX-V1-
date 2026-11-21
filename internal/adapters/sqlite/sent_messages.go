package sqlite

import (
	"context"
	"errors"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
)

// RecordSentMessage stores a message the bot sent, so /del can revoke it later.
func (store *Store) RecordSentMessage(ctx context.Context, key agent.Key, providerMessageID string) error {
	if providerMessageID == "" {
		return agent.NewError(agent.ErrorInvalidArgument, "record sent message", errors.New("provider message ID is required"))
	}
	_, err := store.db.ExecContext(ctx, `INSERT OR IGNORE INTO sent_messages(
    tenant_id, account_id, chat_id, provider_message_id, sent_at_ms
  ) VALUES (?, ?, ?, ?, ?)`,
		key.TenantID.String(), key.AccountID.String(), key.ChatID.String(),
		providerMessageID, store.clock.Now().UTC().UnixMilli(),
	)
	if err != nil {
		return storageError("record sent message", err)
	}
	return nil
}

// SentMessages lists one chat's recorded bot messages, newest first.
func (store *Store) SentMessages(ctx context.Context, key agent.Key) ([]string, error) {
	rows, err := store.read.QueryContext(ctx, `SELECT provider_message_id FROM sent_messages
  WHERE tenant_id = ? AND account_id = ? AND chat_id = ? ORDER BY sent_at_ms DESC, provider_message_id`,
		key.TenantID.String(), key.AccountID.String(), key.ChatID.String())
	if err != nil {
		return nil, storageError("list sent messages", err)
	}
	defer rows.Close()
	var messages []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, storageError("scan sent message", err)
		}
		messages = append(messages, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, storageError("iterate sent messages", err)
	}
	return messages, nil
}

// DeleteSentMessage forgets one recorded bot message.
func (store *Store) DeleteSentMessage(ctx context.Context, key agent.Key, providerMessageID string) error {
	_, err := store.db.ExecContext(ctx, `DELETE FROM sent_messages
  WHERE tenant_id = ? AND account_id = ? AND chat_id = ? AND provider_message_id = ?`,
		key.TenantID.String(), key.AccountID.String(), key.ChatID.String(), providerMessageID)
	if err != nil {
		return storageError("delete sent message", err)
	}
	return nil
}
