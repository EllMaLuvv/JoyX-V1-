package commands

import (
	"context"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func init() {
	register(command.Command{
		Name:        "restart",
		Permission:  "owner and !fromMe",
		Description: "Restarts the bot. Owner only. Joy's old .reset did the same.",
		DeniedReply: "Only the owner can restart the bot.",
		Run:         runRestart,
	})
}

const restartUsage = "Usage: /restart\nThe bot stops and has to be started again."

func runRestart(ctx context.Context, c *command.Context) error {
	if c.HasArgs {
		return c.Reply(ctx, restartUsage)
	}
	// The reply goes out first: shutting down cancels the runtime, and a
	// message planned after it would never be sent.
	if err := c.Reply(ctx, "Restarting…"); err != nil {
		return err
	}
	return c.Restart(ctx)
}
