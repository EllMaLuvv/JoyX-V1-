package commands

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func init() {
	register(command.Command{
		Name:    "download",
		Aliases: []string{"tt", "tiktok", "ig", "instagram", "fb", "facebook", "dl"},
		// The assistant also runs this one on a member's request ("download
		// this"), and a bot-originated call carries fromMe=true, so excluding
		// fromMe would deny exactly the flow the chat asks for.
		Permission:  "public",
		Description: "Downloads a TikTok, Instagram or Facebook video or story at up to 1080p60 and sends it here. /download <link>.",
		Run:         runDownload,
	})
}

const downloadUsage = "Usage: /download <link>\nPaste a TikTok, Instagram or Facebook video link."

func runDownload(ctx context.Context, c *command.Context) error {
	rawURL := strings.TrimSpace(c.Args)
	if rawURL == "" || strings.ContainsAny(rawURL, " \t") {
		return c.Reply(ctx, downloadUsage)
	}
	media, err := c.FetchRemoteMedia(ctx, rawURL)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(media.Path))
	if err := sendMedia(ctx, c, media); err != nil {
		return err
	}
	return c.Reply(ctx, downloadCaption(media))
}

// downloadCaption names the title and the resolution that was actually fetched,
// so a chat can see when a source had no 1080p60 to give.
func downloadCaption(media command.RemoteMedia) string {
	label := fmt.Sprintf("%dx%d", media.Width, media.Height)
	if media.FPS > 0 {
		label += fmt.Sprintf(" %dfps", int(math.Round(media.FPS)))
	}
	title := strings.TrimSpace(media.Title)
	if title == "" {
		return label
	}
	return title + "\n" + label
}
