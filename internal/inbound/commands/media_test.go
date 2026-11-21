package commands

import (
	"context"
	"testing"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// recordingMedia stands in for the provider's media upload.
type recordingMedia struct {
	images    [][]byte
	videos    [][]byte
	mimetypes []string
}

func (recorder *recordingMedia) SendImage(_ context.Context, _ agent.Key, data []byte, mimetype string, _ identity.MessageID) error {
	recorder.images = append(recorder.images, data)
	recorder.mimetypes = append(recorder.mimetypes, mimetype)
	return nil
}

func (recorder *recordingMedia) SendVideo(_ context.Context, _ agent.Key, data []byte, mimetype string, _ identity.MessageID) error {
	recorder.videos = append(recorder.videos, data)
	recorder.mimetypes = append(recorder.mimetypes, mimetype)
	return nil
}

func runMediaCommand(t *testing.T, text string, media *command.Media, sender *recordingMedia, facts command.PermissionFacts) (*recordingText, error) {
	t.Helper()
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse(text)
	if !recognized {
		t.Fatalf("%q is not a registered command", text)
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	store := &mediaStore{}
	downloader := fakeMedia{}
	if media != nil {
		store.payload = []byte(`{}`)
		downloader.media = *media
	}
	replies := &recordingText{}
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: text},
		Facts:    facts,
		Platform: command.Platform{Text: replies, Media: downloader, MediaSender: sender},
		Store:    store,
	})
	return replies, err
}

func TestImageCommandSendsStickerAsPNG(t *testing.T) {
	png := testPNGBytes(t)
	sender := &recordingMedia{}
	replies, err := runMediaCommand(t, "/img", &command.Media{Kind: command.MediaSticker, Data: png, Mimetype: "image/webp"}, sender, command.PermissionFacts{IsPrivate: true})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(sender.images) != 1 {
		t.Fatalf("images = %d; want 1", len(sender.images))
	}
	if sender.mimetypes[0] != "image/png" {
		t.Fatalf("mimetype = %q", sender.mimetypes[0])
	}
	if len(replies.sent) != 0 {
		t.Fatalf("a successful send does not reply with text: %q", replies.sent)
	}
}

func TestImageCommandResendsAnImageUnchanged(t *testing.T) {
	png := testPNGBytes(t)
	sender := &recordingMedia{}
	if _, err := runMediaCommand(t, "/img", &command.Media{Kind: command.MediaImage, Data: png, Mimetype: "image/jpeg"}, sender, command.PermissionFacts{IsPrivate: true}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(sender.images) != 1 || sender.mimetypes[0] != "image/jpeg" {
		t.Fatalf("images = %d mimetypes = %q", len(sender.images), sender.mimetypes)
	}
}

func TestImageCommandWithoutMediaExplainsItself(t *testing.T) {
	sender := &recordingMedia{}
	replies, err := runMediaCommand(t, "/img", nil, sender, command.PermissionFacts{IsPrivate: true})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(sender.images) != 0 || len(replies.sent) == 0 {
		t.Fatalf("sent = %q images = %d", replies.sent, len(sender.images))
	}
}

func TestResendViewOnceSendsTheMediaAgain(t *testing.T) {
	sender := &recordingMedia{}
	png := testPNGBytes(t)
	if _, err := runMediaCommand(t, "/rvo", &command.Media{Kind: command.MediaImage, Data: png, Mimetype: "image/jpeg"}, sender, command.PermissionFacts{IsOwner: true, IsPrivate: true}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(sender.images) != 1 || sender.mimetypes[0] != "image/jpeg" {
		t.Fatalf("images = %d mimetypes = %q", len(sender.images), sender.mimetypes)
	}
}

func TestResendViewOnceIsOwnerOnly(t *testing.T) {
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse("/rvo")
	if !recognized {
		t.Fatal("/rvo is not registered")
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/rvo"},
		Facts:    command.PermissionFacts{IsAdmin: true, IsGroup: true},
		Platform: command.Platform{Text: &recordingText{}, Media: fakeMedia{}, MediaSender: &recordingMedia{}},
		Store:    &handledStore{},
	})
	if err == nil {
		t.Fatal("a group admin who is not the owner must be refused")
	}
}

func TestDeleteBotMessagesReportsTheCount(t *testing.T) {
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse("/del")
	if !recognized {
		t.Fatal("/del is not registered")
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	replies := &recordingText{}
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/del"},
		Facts:    command.PermissionFacts{IsOwner: true, IsPrivate: true},
		Platform: command.Platform{Text: replies, Revoker: recordingRevoker{count: 3}},
		Store:    &handledStore{},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(replies.sent) != 1 {
		t.Fatalf("sent = %q", replies.sent)
	}
}

// recordingRevoker stands in for the dispatcher's /del work.
type recordingRevoker struct{ count int }

func (recorder recordingRevoker) DeleteBotMessages(context.Context, agent.Key) (int, error) {
	return recorder.count, nil
}

func TestRestartIsOwnerOnly(t *testing.T) {
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse("/restart")
	if !recognized {
		t.Fatal("/restart is not registered")
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	restarts := 0
	replies := &recordingText{}
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/restart"},
		Facts:    command.PermissionFacts{IsOwner: true, IsPrivate: true},
		Platform: command.Platform{Text: replies, Restarter: restartCounter{&restarts}},
		Store:    &handledStore{},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if restarts != 1 || len(replies.sent) != 1 {
		t.Fatalf("restarts = %d sent = %q", restarts, replies.sent)
	}
}

type restartCounter struct{ count *int }

func (counter restartCounter) Restart(context.Context) error {
	*counter.count++
	return nil
}
