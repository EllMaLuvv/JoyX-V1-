package inbound

import (
	"context"
	"math/rand"
	"regexp"
	"strings"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/action"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// tifosiJokes are Joy's Ferrari jokes, kept verbatim so the merged bot still
// answers the way the group expects.
var tifosiJokes = []string{
	"Ferrari lagi? Waduh, mereka aja juara dunia paling banyak kalah :)) 🏎️💨",
	"Kabar baik untuk Tifosi: kalau Ferrari gak juara, minimal menang adu nasib. 😆",
	"Ferrari: strategi oke, hasil... sama aja. Tifosi setia kayak WiFi gratis! 📶",
	"Ferrari itu seperti langganan warung langganan: gak pernah ganti menu. 😅",
	"Tifosi: kalau kamu sebut nama Ferrari, kamu dapet boost moral! Sayangnya bukan di mobilnya. 😂",
	"Ferrari menang terakhir kali? Mungkin harus tanya orang tua kamu dulu. 🤣",
	"Kabar gembira Tifosi: kalau menang terus nanti bosan! Santai aja... eh malah belum hujan. ☔",
	"Ferrari di track: merah... merah... merah terus! Yang kuning? Itu lampu kuning di tikungan. 😬",
}

// autoReplyRule is one keyword and the reply it earns.
type autoReplyRule struct {
	pattern *regexp.Regexp
	reply   func() string
}

// autoReplyRules are Joy's built-in keyword replies, in the order they are
// tried. A message that matches one is answered and never reaches the model.
var autoReplyRules = []autoReplyRule{
	{pattern: regexp.MustCompile(`\b(ijin|izin)\s+kompe\b`), reply: func() string { return "Mode On Aktif DiNyalakan " }},
	{pattern: regexp.MustCompile(`\btifosi\b`), reply: func() string { return tifosiJokes[rand.Intn(len(tifosiJokes))] }},
}

// matchAutoReply reports the reply a message earns, if any.
func matchAutoReply(text string) (string, bool) {
	lowered := strings.ToLower(text)
	for _, rule := range autoReplyRules {
		if rule.pattern.MatchString(lowered) {
			return rule.reply(), true
		}
	}
	return "", false
}

// sendAutoReply answers message directly, the way Joy did, and records that it
// was answered without a model turn.
func (dispatcher *Dispatcher) sendAutoReply(ctx context.Context, key agent.Key, message conversation.IncomingMessage, text string) error {
	actionID, err := identity.NewActionID()
	if err != nil {
		return agent.NewError(agent.ErrorInternal, "send automatic reply", err)
	}
	if _, err := dispatcher.platform.Text.SendText(ctx, action.SendTextRequest{Key: key, ActionID: actionID, Text: text}); err != nil {
		return err
	}
	return dispatcher.ignore(ctx, message, IgnoreAutoReplied)
}
