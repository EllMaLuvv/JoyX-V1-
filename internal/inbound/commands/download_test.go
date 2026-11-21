package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// recordingRemoteMedia stands in for the downloader port.
type recordingRemoteMedia struct {
	media command.RemoteMedia
	err   error
	calls []string
}

func (recorder *recordingRemoteMedia) Fetch(_ context.Context, rawURL string) (command.RemoteMedia, error) {
	recorder.calls = append(recorder.calls, rawURL)
	return recorder.media, recorder.err
}

func TestDownloadCommandSendsTheVideo(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "media.mp4")
	if err := os.WriteFile(path, []byte("video-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	fetcher := &recordingRemoteMedia{media: command.RemoteMedia{
		Path: path, Title: "My Video", Width: 1920, Height: 1080, FPS: 60, Mimetype: "video/mp4",
	}}
	sender := &recordingMedia{}
	replies := &recordingText{}
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse("/download https://www.tiktok.com/@user/video/123")
	if !recognized {
		t.Fatal("/download is not registered")
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/download https://www.tiktok.com/@user/video/123"},
		Facts:    command.PermissionFacts{IsPrivate: true},
		Platform: command.Platform{Text: replies, MediaSender: sender, RemoteMedia: fetcher},
		Store:    &handledStore{},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(fetcher.calls) != 1 {
		t.Fatalf("fetches = %q", fetcher.calls)
	}
	if len(sender.videos) != 1 || string(sender.videos[0]) != "video-bytes" {
		t.Fatalf("videos = %d", len(sender.videos))
	}
	if len(replies.sent) != 1 || replies.sent[0] != "My Video\n1920x1080 60fps" {
		t.Fatalf("sent = %q", replies.sent)
	}
}

func TestDownloadCommandRefusesAMissingLink(t *testing.T) {
	fetcher := &recordingRemoteMedia{}
	replies := &recordingText{}
	registry := builtinRegistry(t)
	request, _, _ := registry.Parse("/download")
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	if err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/download"},
		Facts:    command.PermissionFacts{IsPrivate: true},
		Platform: command.Platform{Text: replies, RemoteMedia: fetcher},
		Store:    &handledStore{},
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(fetcher.calls) != 0 || len(replies.sent) == 0 {
		t.Fatalf("fetches = %q sent = %q", fetcher.calls, replies.sent)
	}
}

func TestDownloadCommandSurfacesAFetchFailure(t *testing.T) {
	fetcher := &recordingRemoteMedia{err: errors.New("login required")}
	registry := builtinRegistry(t)
	request, _, _ := registry.Parse("/dl https://www.instagram.com/reel/ABC/")
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/dl https://www.instagram.com/reel/ABC/"},
		Facts:    command.PermissionFacts{IsPrivate: true},
		Platform: command.Platform{Text: &recordingText{}, RemoteMedia: fetcher},
		Store:    &handledStore{},
	})
	if err == nil {
		t.Fatal("a failed download must reach the caller")
	}
}

// The assistant runs /download when a member asks it to, and bot-originated
// invocations carry fromMe=true. Denying those would make the feature look
// broken to exactly the person who asked.
func TestDownloadCommandIsAllowedForTheAssistant(t *testing.T) {
	fetcher := &recordingRemoteMedia{err: errors.New("login required")}
	registry := builtinRegistry(t)
	request, _, _ := registry.Parse("/tt https://www.tiktok.com/@user/video/123")
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: "/tt https://www.tiktok.com/@user/video/123"},
		Facts:    command.PermissionFacts{FromMe: true, IsPrivate: true},
		Platform: command.Platform{Text: &recordingText{}, RemoteMedia: fetcher},
		Store:    &handledStore{},
	})
	if err == nil {
		t.Fatal("the download must still run (and report its own failure)")
	}
	if agent.IsCode(err, agent.ErrorPermissionDenied) {
		t.Fatalf("bot-originated download was denied: %v", err)
	}
	if len(fetcher.calls) != 1 {
		t.Fatalf("fetches = %q, want one attempt", fetcher.calls)
	}
}
