package inbound

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/action"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/downloader"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// urlPattern finds the first http(s) URL in a message.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// firstSupportedLink reports the first TikTok, Instagram or Facebook link in a
// message, so a chat that drops one gets the video without typing a command.
func firstSupportedLink(text string) (string, bool) {
	for _, candidate := range urlPattern.FindAllString(text, -1) {
		candidate = strings.TrimRight(candidate, ".,;:!?)]}")
		if downloader.AllowedHost(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// autoDownload fetches a link a chat sent and answers with the video. The
// message is marked handled at once, so a restart never downloads it twice;
// the work itself runs beside the turn queue because it can take minutes.
func (dispatcher *Dispatcher) autoDownload(ctx context.Context, key agent.Key, message conversation.IncomingMessage, rawURL string) error {
	if err := dispatcher.ignore(ctx, message, IgnoreLinkAutoDownloaded); err != nil {
		return err
	}
	if dispatcher.platform.RemoteMedia == nil || dispatcher.platform.MediaSender == nil {
		return agent.NewError(agent.ErrorUnavailable, "download link",
			errors.New("the downloader is not configured in this host"))
	}
	dispatcher.mu.Lock()
	if dispatcher.ctx.Err() != nil {
		dispatcher.mu.Unlock()
		return nil
	}
	dispatcher.turns.Add(1)
	dispatcher.mu.Unlock()
	go func() {
		defer dispatcher.turns.Done()
		dispatcher.runAutoDownload(dispatcher.ctx, key, message.ID, rawURL)
	}()
	return nil
}

func (dispatcher *Dispatcher) runAutoDownload(ctx context.Context, key agent.Key, quoted identity.MessageID, rawURL string) {
	media, err := dispatcher.platform.RemoteMedia.Fetch(ctx, rawURL)
	if err != nil {
		dispatcher.options.Report(err)
		dispatcher.sendDownloadFailure(ctx, key, err)
		return
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(media.Path)) }()
	data, err := os.ReadFile(media.Path)
	if err != nil {
		wrapped := agent.NewError(agent.ErrorStorageFailure, "read downloaded video", err)
		dispatcher.options.Report(wrapped)
		dispatcher.sendDownloadFailure(ctx, key, wrapped)
		return
	}
	if strings.HasPrefix(media.Mimetype, "image/") {
		if err := dispatcher.platform.MediaSender.SendImage(ctx, key, data, media.Mimetype, quoted); err != nil {
			dispatcher.options.Report(err)
		}
		return
	}
	if err := dispatcher.sendDownloadFile(ctx, key, media, quoted); err != nil {
		dispatcher.options.Report(err)
		return
	}
	actionID, err := identity.NewActionID()
	if err != nil {
		dispatcher.options.Report(agent.NewError(agent.ErrorInternal, "send download caption", err))
		return
	}
	_, err = dispatcher.platform.Text.SendText(ctx, action.SendTextRequest{
		Key: key, ActionID: actionID, QuotedMessageID: quoted, Text: downloadCaption(media),
	})
	if err != nil {
		dispatcher.options.Report(err)
	}
}

// sendDownloadFile delivers a fetched download as a video message, so it plays
// inline in the chat instead of arriving as a document.
func (dispatcher *Dispatcher) sendDownloadFile(ctx context.Context, key agent.Key, media command.RemoteMedia, quoted identity.MessageID) error {
	data, err := os.ReadFile(media.Path)
	if err != nil {
		return agent.NewError(agent.ErrorStorageFailure, "read downloaded video", err)
	}
	return dispatcher.platform.MediaSender.SendVideo(ctx, key, data, media.Mimetype, quoted)
}

func (dispatcher *Dispatcher) sendDownloadFailure(ctx context.Context, key agent.Key, cause error) {
	if dispatcher.platform.Text == nil {
		return
	}
	actionID, err := identity.NewActionID()
	if err != nil {
		return
	}
	message := "Maaf, videonya gagal diunduh: " + downloadFailure(cause)
	_, _ = dispatcher.platform.Text.SendText(ctx, action.SendTextRequest{Key: key, ActionID: actionID, Text: message})
}

// downloadCaption names what was actually fetched, so a chat sees when the
// source had nothing better than 720p30.
func downloadCaption(media command.RemoteMedia) string {
	label := downloadResolutionLabel(media.Width, media.Height, media.FPS)
	title := strings.TrimSpace(media.Title)
	if title == "" {
		return label
	}
	return title + "\n" + label
}

// downloadResolutionLabel renders what the source actually offered.
func downloadResolutionLabel(width, height int, fps float64) string {
	label := fmt.Sprintf("%dx%d", width, height)
	if fps > 0 {
		label += fmt.Sprintf(" %dfps", int(math.Round(fps)))
	}
	return label
}

func downloadFailure(cause error) string {
	message := strings.TrimSpace(cause.Error())
	if index := strings.LastIndexByte(message, ':'); index >= 0 && index+1 < len(message) {
		message = strings.TrimSpace(message[index+1:])
	}
	if len(message) > 200 {
		message = message[:200]
	}
	if message == "" {
		return "tautan privat, sudah dihapus, atau butuh login."
	}
	return message + "."
}
