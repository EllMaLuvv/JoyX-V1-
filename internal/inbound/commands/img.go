package commands

import (
	"context"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/sticker"
)

func init() {
	register(command.Command{
		Name:        "img",
		Aliases:     []string{"gambar"},
		Permission:  "public and !fromMe",
		Description: "Sends a sticker back as a PNG image. Reply to the sticker (or an image) with /img.",
		Run:         runImage,
	})
}

const imageUsage = "Reply to a sticker with /img and I will send it back as an image."

func runImage(ctx context.Context, c *command.Context) error {
	media, err := c.Media(ctx)
	if agent.IsCode(err, agent.ErrorNotFound) {
		return c.Reply(ctx, imageUsage)
	}
	if err != nil {
		return err
	}
	if len(media.Lottie) > 0 {
		return c.Reply(ctx, "That is an animated WhatsApp sticker, which cannot be turned into an image.")
	}
	switch media.Kind {
	case command.MediaSticker:
		png, err := sticker.ToPNG(media.Data)
		if err != nil {
			return c.Reply(ctx, sticker.Problem(err))
		}
		return c.SendImage(ctx, png, "image/png")
	case command.MediaImage:
		mimetype := media.Mimetype
		if mimetype == "" {
			mimetype = "image/png"
		}
		return c.SendImage(ctx, media.Data, mimetype)
	default:
		return c.Reply(ctx, imageUsage)
	}
}
