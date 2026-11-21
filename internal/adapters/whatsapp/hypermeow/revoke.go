package hypermeow

import (
	"context"
	"errors"

	"github.com/polymorfa/hypermeow/types"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
)

// RevokeOwnMessage revokes one message the bot itself sent, for everyone. It
// backs /del, which cleans up after the bot in one chat.
func (adapter *Adapter) RevokeOwnMessage(ctx context.Context, key agent.Key, providerMessageID string) error {
	if !adapter.Ready() {
		return agent.NewError(agent.ErrorNotReady, "revoke WhatsApp message", errors.New("account is not connected"))
	}
	if providerMessageID == "" {
		return agent.NewError(agent.ErrorInvalidArgument, "revoke WhatsApp message", errors.New("provider message ID is required"))
	}
	target, err := adapter.resolveChatTarget(ctx, key)
	if err != nil {
		return err
	}
	own := adapter.ownJID()
	if own.IsEmpty() {
		return agent.NewError(agent.ErrorIntegrityFailure, "revoke WhatsApp message", errors.New("the bot's own address is unknown"))
	}
	requestCtx, cancel := context.WithTimeout(ctx, adapter.sendTimeout)
	defer cancel()
	stripe := adapter.sendStripe(key.ChatID.String())
	stripe.Lock()
	defer stripe.Unlock()
	if _, err := adapter.client.SendMessage(requestCtx, target, adapter.client.BuildRevoke(target, own, types.MessageID(providerMessageID))); err != nil {
		return nativeEffectError(requestCtx, "revoke WhatsApp message", err)
	}
	return nil
}
