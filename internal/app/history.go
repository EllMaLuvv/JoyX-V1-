package app

import (
	"context"
	"time"

	appsqlite "github.com/Chomosuke9/WazzapAgent-Go/internal/adapters/sqlite"
	whatsapp "github.com/Chomosuke9/WazzapAgent-Go/internal/adapters/whatsapp/hypermeow"
)

// pairingHistory feeds the pairing page's message-history box out of the same
// history store the agent reads, so what the page shows is what Joy really sent.
type pairingHistory struct {
	store *appsqlite.Store
}

func (source pairingHistory) RecentMessages(ctx context.Context, since time.Time, limit int) ([]whatsapp.HistoryMessage, error) {
	messages, err := source.store.History().RecentAssistant(ctx, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	recent := make([]whatsapp.HistoryMessage, 0, len(messages))
	for _, message := range messages {
		recent = append(recent, whatsapp.HistoryMessage{
			ChatID: message.ChatID,
			Text:   message.Text,
			At:     message.At,
		})
	}
	return recent, nil
}

// wirePairingHistory hands the page its history source. The web sink is built in
// main before the application exists, so it is reached through the configured
// pairing sinks rather than a constructor argument.
func wirePairingHistory(sinks whatsapp.PairingSink, source whatsapp.HistorySource) {
	if fanout, ok := sinks.(whatsapp.MultiPairingSink); ok {
		for _, sink := range fanout {
			setPairingHistory(sink, source)
		}
		return
	}
	setPairingHistory(sinks, source)
}

func setPairingHistory(sink whatsapp.PairingSink, source whatsapp.HistorySource) {
	if capable, ok := sink.(interface{ SetHistorySource(whatsapp.HistorySource) }); ok {
		capable.SetHistorySource(source)
	}
}
