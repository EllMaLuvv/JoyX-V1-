package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/inbound"
)

// SaveDeadline stores deadline and reports whether it is new. A second
// deadline from the same source message is not stored.
func (store *InboundStore) SaveDeadline(ctx context.Context, deadline inbound.Deadline) (bool, error) {
	if err := deadline.Validate(); err != nil {
		return false, agent.NewError(agent.ErrorInvalidArgument, "save deadline", err)
	}
	result, err := store.db.ExecContext(ctx, `INSERT OR IGNORE INTO deadlines(
    tenant_id, account_id, chat_id, id, source_message_id, name, deadline_ms, created_at_ms
  ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		deadline.Key.TenantID.String(), deadline.Key.AccountID.String(), deadline.Key.ChatID.String(),
		deadline.ID.String(), deadline.Source.String(), deadline.Name,
		deadline.Deadline.UTC().UnixMilli(), store.clock.Now().UTC().UnixMilli(),
	)
	if err != nil {
		return false, storageError("save deadline", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, storageError("inspect deadline save", err)
	}
	return changed == 1, nil
}

// ListDeadlines returns every deadline of the tenant, soonest first.
func (store *InboundStore) ListDeadlines(ctx context.Context, tenantID identity.TenantID) ([]inbound.Deadline, error) {
	rows, err := store.read.QueryContext(ctx, `SELECT account_id, chat_id, id, source_message_id, name, deadline_ms, reminded_ms
  FROM deadlines WHERE tenant_id = ? ORDER BY deadline_ms, id`, tenantID.String())
	if err != nil {
		return nil, storageError("list deadlines", err)
	}
	defer rows.Close()
	var deadlines []inbound.Deadline
	for rows.Next() {
		var account, chat, id, source, name string
		var deadlineMs int64
		var remindedMs sql.NullInt64
		if err := rows.Scan(&account, &chat, &id, &source, &name, &deadlineMs, &remindedMs); err != nil {
			return nil, storageError("scan deadline", err)
		}
		deadline := inbound.Deadline{
			Key:      agent.Key{TenantID: tenantID},
			Name:     name,
			Deadline: time.UnixMilli(deadlineMs).UTC(),
			Reminded: remindedMs.Valid,
		}
		var parseErr error
		if deadline.Key.AccountID, err = identity.ParseAccountID(account); err != nil {
			parseErr = err
		} else if deadline.Key.ChatID, err = identity.ParseChatID(chat); err != nil {
			parseErr = err
		} else if deadline.ID, err = identity.ParseCausationID(id); err != nil {
			parseErr = err
		} else if deadline.Source, err = identity.ParseMessageID(source); err != nil {
			parseErr = err
		}
		if parseErr != nil {
			return nil, agent.NewError(agent.ErrorIntegrityFailure, "decode deadline", parseErr)
		}
		deadlines = append(deadlines, deadline)
	}
	if err := rows.Err(); err != nil {
		return nil, storageError("iterate deadlines", err)
	}
	return deadlines, nil
}

// DeleteDeadline removes a deadline that has been deleted or has reminded. A
// missing deadline is not an error.
func (store *InboundStore) DeleteDeadline(ctx context.Context, key agent.Key, id identity.CausationID) error {
	if id.IsZero() {
		return agent.NewError(agent.ErrorInvalidArgument, "delete deadline", errors.New("deadline ID is required"))
	}
	_, err := store.db.ExecContext(ctx, `DELETE FROM deadlines WHERE tenant_id = ? AND id = ?`,
		key.TenantID.String(), id.String())
	if err != nil {
		return storageError("delete deadline", err)
	}
	return nil
}

// MarkDeadlineReminded records that a deadline's reminder went out.
func (store *InboundStore) MarkDeadlineReminded(ctx context.Context, id identity.CausationID) error {
	if id.IsZero() {
		return agent.NewError(agent.ErrorInvalidArgument, "mark deadline reminded", errors.New("deadline ID is required"))
	}
	_, err := store.db.ExecContext(ctx, `UPDATE deadlines SET reminded_ms = ? WHERE id = ?`,
		store.clock.Now().UTC().UnixMilli(), id.String())
	if err != nil {
		return storageError("mark deadline reminded", err)
	}
	return nil
}
