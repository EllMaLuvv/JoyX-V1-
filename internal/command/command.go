// Package command is the slash-command framework. Each command lives in one
// file under internal/inbound/commands and registers a Command; the Registry
// routes "/name args" to it, and Context is everything the command may use.
// Commands never import each other, and nothing outside their file has to
// change when one is added.
package command

import (
	"context"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/action"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/policy"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/sticker"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/subagent"
)

// Command is one slash command.
type Command struct {
	// Name is the canonical token: lowercase, without the slash.
	Name    string
	Aliases []string
	// Permission is a boolean expression over public, owner, admin, group,
	// private and fromMe, evaluated for every invocation and button tap.
	Permission  string
	Description string
	// DeniedReply is sent when Permission denies the sender. Optional.
	DeniedReply string
	// Run executes the command. Taps on buttons and menus this command sent
	// arrive here too, as "/<Name> <Button.Args>".
	Run func(ctx context.Context, c *Context) error
}

// Button is a quick-reply button, or a row in a Menu. A tap runs
// "/<Command> <Args>" through the router, so the tapper's own permission is
// checked as if they typed it.
type Button struct {
	Label string
	Args  string
	// Command is the command a tap runs; empty means the one that sent it.
	Command string
	// Description is a second line under the label, shown only in a Menu.
	Description string
}

// Menu is a list button: tapping Title opens Options, and picking one
// runs it like a Button tap.
type Menu struct {
	Title   string
	Options []Button
}

// PermissionFacts are the trusted facts a Permission expression is evaluated
// against. They come from the policy boundary, never from message text.
type PermissionFacts = policy.PermissionFacts

// OutboundFile is a local file to send as its own message. An empty Kind is
// delivered as a document, which WhatsApp hands over byte for byte — a video
// message would be recompressed on the way out.
type OutboundFile struct {
	Path string
	Name string
	Mime string
	Kind string
}

// FileSender sends one local file and returns the provider receipt.
type FileSender interface {
	SendFile(ctx context.Context, key agent.Key, file OutboundFile) (string, error)
}

// Platform holds the provider ports the process lends to commands. A nil
// port means the capability is unavailable in this host.
type Platform struct {
	Text    TextSender
	Buttons ButtonSender
	Group   GroupModerator
	Media   MediaDownloader
	// MediaSender uploads and sends raw image or video bytes (/img, /rvo).
	MediaSender MediaSender
	// Files sends a local file as its own message, used for downloads so they
	// reach the chat uncompressed.
	Files FileSender
	// Revoker revokes the bot's own messages, for /del.
	Revoker MessageRevoker
	// RemoteMedia downloads a public video, for /download.
	RemoteMedia RemoteMediaFetcher
	// Stickers sends stickers; Catalog is each chat's named sticker store.
	Stickers StickerSender
	Catalog  sticker.Catalog
	Tasks    TaskScheduler
	// Memory is each chat's long-term memory (/memory).
	Memory MemoryStore
	// MessageMedia reads the media of a message a command targets, for
	// commands the bot runs, which have no inbox record of their own.
	MessageMedia MessageMediaReader
	// Subagent is the sub-agent switch (/subagent). Nil means no sub-agent
	// service is configured.
	Subagent SubagentControl
	// Deadlines saves and lists the chat's /dd deadline reminders.
	Deadlines DeadlineEditor
	// Restarter ends the process, for /restart.
	Restarter Restarter
	// AssistantName is the bot's configured display name, used to write
	// its "@Name (Bot)" mention.
	AssistantName string
}

// SubagentControl reads and sets whether chats may delegate tasks to the
// sub-agent.
type SubagentControl interface {
	Enabled(ctx context.Context, key agent.Key) bool
	SetEnabled(ctx context.Context, key agent.Key, scope subagent.Scope, enabled bool) error
}

type TextSender interface {
	SendText(context.Context, action.SendTextRequest) (action.SendTextResult, error)
}

type ButtonSender interface {
	SendButtons(context.Context, action.SendButtonsRequest) (action.SendTextResult, error)
}

// GroupModerator is the provider-neutral port for group administration.
// Implementations own resolving agent identities to provider addresses.
type GroupModerator interface {
	// SetGroupAnnounce restricts sending to admins when announce is true.
	SetGroupAnnounce(ctx context.Context, key agent.Key, announce bool) error
	SetGroupDescription(ctx context.Context, key agent.Key, description string) error
	// RevokeGroupMessage deletes a stored message for every group member.
	RevokeGroupMessage(ctx context.Context, key agent.Key, target identity.MessageID) error
	// RemoveGroupMember kicks the member addressed by ref from the group.
	RemoveGroupMember(ctx context.Context, key agent.Key, ref identity.SenderRef) error
	// MuteGroupMember silences ref for minutes starting at now; zero unmutes.
	MuteGroupMember(ctx context.Context, key agent.Key, ref identity.SenderRef, minutes uint32, now time.Time) error
}

// MediaKind is what kind of media a command received.
type MediaKind uint8

const (
	MediaImage MediaKind = iota + 1
	MediaVideo
	MediaSticker
)

// Media is the image, video or sticker a command message carried or replied to.
type Media struct {
	Kind MediaKind
	// Data is the downloaded file. It is empty for a Lottie sticker.
	Data []byte
	// Mimetype is the provider's own type, preserved so a command that
	// resends the bytes (/rvo) keeps it.
	Mimetype string
	// Animated is set for animated stickers and GIF-style videos.
	Animated bool
	// Lottie is set for WhatsApp's Lottie (premium) stickers: the provider
	// payload that resends the sticker unchanged. It is opaque to commands.
	Lottie []byte
}

// MediaDownloader fetches media from a payload the adapter captured with the
// command message.
type MediaDownloader interface {
	DownloadMedia(ctx context.Context, payload []byte) (Media, error)
}

// MediaSender uploads and sends raw image or video bytes, quoting quoted when
// it is set.
type MediaSender interface {
	SendImage(ctx context.Context, key agent.Key, data []byte, mimetype string, quoted identity.MessageID) error
	SendVideo(ctx context.Context, key agent.Key, data []byte, mimetype string, quoted identity.MessageID) error
}

// RemoteMedia is a downloaded video ready to send, from /download.
type RemoteMedia struct {
	Path     string
	Title    string
	Width    int
	Height   int
	FPS      float64
	Mimetype string
}

// FileName is a WhatsApp-safe name for the fetched file, so the recipient sees
// what they saved instead of a random string. The extension comes from the
// finished file, because yt-dlp may have merged or converted it.
func (media RemoteMedia) FileName() string {
	builder := strings.Builder{}
	for _, runeValue := range strings.TrimSpace(media.Title) {
		switch {
		case unicode.IsLetter(runeValue), unicode.IsDigit(runeValue):
			builder.WriteRune(runeValue)
		case runeValue == ' ', runeValue == '-', runeValue == '_':
			builder.WriteRune(runeValue)
		default:
			builder.WriteRune('-')
		}
	}
	name := strings.Join(strings.Fields(builder.String()), "-")
	if len(name) > 48 {
		name = strings.TrimSpace(name[:48])
	}
	if name == "" {
		name = "joyx-video"
	}
	return name + strings.ToLower(filepath.Ext(media.Path))
}

// RemoteMediaFetcher downloads a public TikTok, Instagram or Facebook video.
// The caller removes Path.
type RemoteMediaFetcher interface {
	Fetch(ctx context.Context, rawURL string) (RemoteMedia, error)
}

// MessageRevoker deletes the bot's own messages in a chat, for everyone, so
// /del can clean up after the bot.
type MessageRevoker interface {
	DeleteBotMessages(ctx context.Context, key agent.Key) (int, error)
}

// MessageMediaReader returns the media payload captured with an incoming
// message, or an agent.ErrorNotFound error when it carried none.
type MessageMediaReader interface {
	ReadMessageMedia(ctx context.Context, key agent.Key, message identity.MessageID) ([]byte, error)
}

// MemoryStore is each chat's long-term memory: notes the bot sees on every
// turn.
type MemoryStore interface {
	// AddMemory saves text. Text the chat already has is not saved twice:
	// the existing memory comes back with added false. A full memory
	// returns an agent.ErrorResourceExhausted error.
	AddMemory(ctx context.Context, key agent.Key, text string) (memory agent.Memory, added bool, err error)
	// Memories lists the chat's memories, oldest first.
	Memories(ctx context.Context, key agent.Key) ([]agent.Memory, error)
	// DeleteMemories deletes the memories with these codes and returns the
	// ones it deleted; unknown codes are skipped.
	DeleteMemories(ctx context.Context, key agent.Key, codes []string) ([]agent.Memory, error)
}

// StickerSender sends a sticker to a chat, quoting quoted when it is set.
type StickerSender interface {
	SendSticker(ctx context.Context, key agent.Key, sticker sticker.Sticker, quoted identity.MessageID) error
}

// TaskScheduler runs a prompt as an AI turn in a chat later, once or every
// day. Tasks are saved, so they still run after a restart. source is the
// command message that asked for one: scheduling twice for one source keeps
// one task, and the second call returns a zero Task.
type TaskScheduler interface {
	ScheduleTask(ctx context.Context, key agent.Key, source identity.MessageID, fireAt time.Time, prompt string) (Task, error)
	// ScheduleDailyTask runs prompt every day at minute (minutes after
	// midnight, in the bot's time zone).
	ScheduleDailyTask(ctx context.Context, key agent.Key, source identity.MessageID, minute int, prompt string) (Task, error)
	// Tasks lists the chat's tasks, soonest first.
	Tasks(ctx context.Context, key agent.Key) ([]Task, error)
	// CancelTask deletes the chat's one-off or daily task with this code. It
	// reports false when there is none.
	CancelTask(ctx context.Context, key agent.Key, code string, daily bool) (bool, error)
}

// Task is a saved task as commands show it.
type Task struct {
	// Code is the task's short ID, typed to delete it.
	Code   string
	Prompt string
	// FireAt is the next run, in the bot's time zone.
	FireAt time.Time
	Daily  bool
}

// Deadline is a saved /dd deadline as commands show it.
type Deadline struct {
	// Code is the deadline's short ID, typed to delete it.
	Code     string
	Name     string
	Deadline time.Time
	// Reminded is set once the day-before reminder has gone out.
	Reminded bool
}

// DeadlineEditor saves, lists and deletes a chat's deadline reminders.
type DeadlineEditor interface {
	AddDeadline(ctx context.Context, key agent.Key, source identity.MessageID, name string, deadline time.Time) (Deadline, error)
	// Deadlines lists the chat's deadlines, soonest first.
	Deadlines(ctx context.Context, key agent.Key) ([]Deadline, error)
	// DeleteDeadline removes the deadline with this short code. It reports
	// false when there is none.
	DeleteDeadline(ctx context.Context, key agent.Key, code string) (bool, error)
}

// Restarter ends the process, for /restart. A supervisor, or the person who
// started the bot, runs it again.
type Restarter interface {
	Restart(ctx context.Context) error
}

// Store is the durable inbox a command message came from. The registry uses
// it to mark the message handled. Commands never call it.
type Store interface {
	MarkCommandHandled(context.Context, conversation.IncomingMessage) error
	RawQuotedMessageReader
	// ReadCommandMedia returns the media payload captured with the command,
	// or an agent.ErrorNotFound error when it carried none.
	ReadCommandMedia(context.Context, conversation.IncomingMessage) ([]byte, error)
}

type Observer interface {
	ObserveHistoryReset()
}

// Invocation is what the host knows about one command message.
type Invocation struct {
	Agent    *agent.Agent
	Config   agent.ConfigSnapshot
	Message  conversation.IncomingMessage
	Facts    PermissionFacts
	Platform Platform
	// Store is nil for commands that did not come from the durable inbox,
	// such as commands issued by the model inside a reply.
	Store    Store
	Observer Observer
	// Notices, when set, collects the command's notices and text replies
	// instead of sending them, so the commands the bot runs in one reply
	// are reported in one message.
	Notices *NoticeBatch
}
