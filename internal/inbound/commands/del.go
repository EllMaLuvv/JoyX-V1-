package commands

import (
	"context"
	"fmt"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func init() {
	register(command.Command{
		Name:        "del",
		Aliases:     []string{"delete", "bersihkan"},
		Permission:  "owner and !fromMe",
		Description: "Deletes the bot's own messages in this chat for everyone. Owner only.",
		DeniedReply: "Only the owner can delete the bot's messages.",
		Run:         runDeleteBotMessages,
	})
}

const deleteUsage = "Usage: /del\nIt deletes the messages the bot sent in this chat, for everyone."

func runDeleteBotMessages(ctx context.Context, c *command.Context) error {
	if c.HasArgs {
		return c.Reply(ctx, deleteUsage)
	}
	revoked, err := c.DeleteBotMessages(ctx)
	if err != nil {
		return err
	}
	if revoked == 0 {
		return c.Reply(ctx, "There were no bot messages to delete in this chat.")
	}
	return c.Reply(ctx, fmt.Sprintf("Deleted %d of the bot's messages in this chat.", revoked))
}
