package commands

import (
	"context"
	"os"
	"strings"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

// sendMedia delivers a downloaded file as an image or a video, decided by the
// mimetype the downloader read off the finished file. An Instagram story can be
// either, and sending a photo through SendVideo would fail at the platform.
func sendMedia(ctx context.Context, c *command.Context, media command.RemoteMedia) error {
	data, err := os.ReadFile(media.Path)
	if err != nil {
		return agent.NewError(agent.ErrorStorageFailure, "read downloaded media", err)
	}
	if strings.HasPrefix(media.Mimetype, "image/") {
		return c.SendImage(ctx, data, media.Mimetype)
	}
	return c.SendVideo(ctx, data, media.Mimetype)
}
