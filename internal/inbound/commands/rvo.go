package commands

import (
	"context"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func init() {
	register(command.Command{
		Name:        "rvo",
		Aliases:     []string{"resend-view-once"},
		Permission:  "owner and !fromMe",
		Description: "Resends a view-once image or video as a normal message. Reply to the view-once message with /rvo. Owner only.",
		DeniedReply: "Only the owner can resend a view-once message.",
		Run:         runResendViewOnce,
	})
}

const viewOnceUsage = "Reply to the view-once image or video with /rvo and I will send it again as a normal message."

func runResendViewOnce(ctx context.Context, c *command.Context) error {
	media, err := c.Media(ctx)
	if agent.IsCode(err, agent.ErrorNotFound) {
		return c.Reply(ctx, viewOnceUsage)
	}
	if err != nil {
		return err
	}
	switch media.Kind {
	case command.MediaVideo:
		mimetype := media.Mimetype
		if mimetype == "" {
			mimetype = "video/mp4"
		}
		return c.SendVideo(ctx, media.Data, mimetype)
	case command.MediaImage:
		mimetype := media.Mimetype
		if mimetype == "" {
			mimetype = "image/jpeg"
		}
		return c.SendImage(ctx, media.Data, mimetype)
	default:
		return c.Reply(ctx, viewOnceUsage)
	}
}
