package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
)

func init() {
	register(command.Command{
		Name:        "dd",
		Aliases:     []string{"deadline", "tugas"},
		Permission:  "public and !fromMe",
		Description: "Saves a task deadline and reminds the chat the day before. /dd <name> <date> with a date like 07-10-2026. /dd alone lists them, /dd del <code> removes one.",
		Run:         runDeadline,
	})
}

const deadlineUsage = "Usage: /dd <task name> <date>\n" +
	"The date is dd-mm-yyyy, dd/mm/yyyy or yyyy-mm-dd. Example: /dd Makalah Basis Data 07-10-2026\n" +
	"/dd lists this chat's deadlines. /dd del <code> removes one."

func runDeadline(ctx context.Context, c *command.Context) error {
	args := strings.TrimSpace(c.Args)
	if args == "" {
		return replyDeadlineList(ctx, c)
	}
	if name, code, ok := parseDeadlineDelete(args); ok {
		deleted, err := c.DeleteDeadline(ctx, code)
		if err != nil {
			return err
		}
		if !deleted {
			return c.Reply(ctx, "No deadline with code "+code+" in this chat.")
		}
		return c.Notice(ctx, command.Notice{Icon: "🗓️", Kind: "Deadline", Change: command.NoticeRemoved,
			Item: name, Text: "Deadline " + name + " was removed."})
	}
	name, at, err := parseDeadlineArgs(args)
	if err != nil {
		return c.Reply(ctx, deadlineUsage)
	}
	deadline, err := c.AddDeadline(ctx, name, at)
	if err != nil {
		return err
	}
	return c.Notice(ctx, command.Notice{Icon: "🗓️", Kind: "Deadline", Change: command.NoticeAdded,
		Item: name, Text: fmt.Sprintf("Deadline %s saved for %s (code %s).", name, formatDeadlineListDate(deadline.Deadline), deadline.Code)})
}

// parseDeadlineDelete reads "/dd del <code>".
func parseDeadlineDelete(args string) (name, code string, ok bool) {
	fields := strings.Fields(args)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "del") {
		return "", "", false
	}
	return "", fields[1], true
}

// parseDeadlineArgs splits "/dd <name> <date>": the last word must be a date,
// everything before it is the task name.
func parseDeadlineArgs(args string) (string, time.Time, error) {
	fields := strings.Fields(args)
	if len(fields) < 2 {
		return "", time.Time{}, fmt.Errorf("a task name and a date are required")
	}
	raw := fields[len(fields)-1]
	at, err := parseDeadlineDate(raw)
	if err != nil {
		return "", time.Time{}, err
	}
	name := strings.Join(fields[:len(fields)-1], " ")
	if strings.TrimSpace(name) == "" {
		return "", time.Time{}, fmt.Errorf("a task name is required")
	}
	return name, at, nil
}

// parseDeadlineDate reads dd-mm-yyyy, dd/mm/yyyy or yyyy-mm-dd as the end of
// that day in the chat's time zone.
func parseDeadlineDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	var year, month, day int
	var err error
	switch {
	case strings.Count(value, "-") == 2 && len(value) >= 8 && value[4] == '-':
		_, err = fmt.Sscanf(value, "%d-%d-%d", &year, &month, &day)
		if err != nil {
			return time.Time{}, fmt.Errorf("date %q is not yyyy-mm-dd", raw)
		}
	case strings.Count(value, "-") == 2:
		_, err = fmt.Sscanf(value, "%d-%d-%d", &day, &month, &year)
		if err != nil {
			return time.Time{}, fmt.Errorf("date %q is not dd-mm-yyyy", raw)
		}
	case strings.Count(value, "/") == 2:
		_, err = fmt.Sscanf(value, "%d/%d/%d", &day, &month, &year)
		if err != nil {
			return time.Time{}, fmt.Errorf("date %q is not dd/mm/yyyy", raw)
		}
	default:
		return time.Time{}, fmt.Errorf("date %q is not dd-mm-yyyy, dd/mm/yyyy or yyyy-mm-dd", raw)
	}
	if year < 100 {
		year += 2000
	}
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("date %q is not a real calendar date", raw)
	}
	location := time.Local
	return time.Date(year, time.Month(month), day, 23, 59, 0, 0, location), nil
}

func replyDeadlineList(ctx context.Context, c *command.Context) error {
	deadlines, err := c.Deadlines(ctx)
	if err != nil {
		return err
	}
	pending := make([]command.Deadline, 0, len(deadlines))
	for _, deadline := range deadlines {
		if !deadline.Reminded {
			pending = append(pending, deadline)
		}
	}
	if len(pending) == 0 {
		return c.Reply(ctx, "No deadlines recorded in this chat yet.\n\n"+deadlineUsage)
	}
	lines := make([]string, 0, len(pending))
	for index, deadline := range pending {
		lines = append(lines, fmt.Sprintf("%d. %s — %s (code %s)", index+1, deadline.Name, formatDeadlineListDate(deadline.Deadline), deadline.Code))
	}
	return c.Reply(ctx, "Deadlines in this chat:\n\n"+strings.Join(lines, "\n"))
}

// formatDeadlineListDate renders a deadline date as "07-10-2026".
func formatDeadlineListDate(value time.Time) string {
	return value.Format("02-01-2006")
}
