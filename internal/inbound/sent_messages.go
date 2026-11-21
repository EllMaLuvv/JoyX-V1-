package inbound

import (
	"context"
	"errors"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
)

// errMessageRevokerMissing says this host cannot revoke the bot's messages.
var errMessageRevokerMissing = errors.New("message revoker is not available in this host")

// SentMessageStore remembers what the bot sent, so /del can revoke it.
type SentMessageStore interface {
	SentMessages(context.Context, agent.Key) ([]string, error)
	DeleteSentMessage(context.Context, agent.Key, string) error
}

// DeleteBotMessages revokes every message the bot sent in one chat, for
// everyone, and returns how many were revoked. A message that WhatsApp can no
// longer revoke is skipped; an account that is offline stops the run so a
// later attempt can still clean up. It implements command.MessageRevoker.
func (dispatcher *Dispatcher) DeleteBotMessages(ctx context.Context, key agent.Key) (int, error) {
	if dispatcher.options.MessageRevoker == nil {
		return 0, agent.NewError(agent.ErrorUnavailable, "delete the bot's messages", errMessageRevokerMissing)
	}
	recorded, err := dispatcher.store.SentMessages(ctx, key)
	if err != nil {
		return 0, err
	}
	revoked := 0
	for _, providerMessageID := range recorded {
		if err := dispatcher.options.MessageRevoker.RevokeOwnMessage(ctx, key, providerMessageID); err != nil {
			if agent.IsCode(err, agent.ErrorNotReady) {
				return revoked, err
			}
			// The message is gone, is too old to revoke, or was never stored;
			// the rest may still go.
			continue
		}
		revoked++
		if err := dispatcher.store.DeleteSentMessage(ctx, key, providerMessageID); err != nil {
			return revoked, err
		}
	}
	return revoked, nil
}
