package inbound

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	builtincommands "github.com/Chomosuke9/WazzapAgent-Go/internal/inbound/commands"
)

// dotAliases are Joy's dot commands and the slash command each one runs. Joy,
// the bot this build replaces, answered ".s", ".del", ".reset", ".img" and
// ".rvo"; keeping the habit costs nothing and regulars do not have to relearn.
var dotAliases = map[string]string{
	"s":     "sticker",
	"del":   "del",
	"reset": "restart",
	"img":   "img",
	"rvo":   "rvo",
}

// dotDeadline matches Joy's ".dd-Name-Date", where the date is the last
// dash-separated piece.
var dotDeadline = regexp.MustCompile(`(?i)^dd-(.+)-(\d{1,2}[-/]\d{1,2}[-/]\d{2,4}|\d{4}-\d{1,2}-\d{1,2})$`)

// normalizeDotCommand rewrites a leading-dot command into the slash form the
// registry understands. Anything else is returned unchanged, so ordinary text
// that happens to start with a dot still reaches the agent.
func normalizeDotCommand(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "..") {
		return text
	}
	rest := trimmed[1:]
	if match := dotDeadline.FindStringSubmatch(rest); match != nil {
		return "/dd " + strings.TrimSpace(match[1]) + " " + match[2]
	}
	token, remainder, found := strings.Cut(rest, " ")
	if target, ok := dotAliases[strings.ToLower(token)]; ok {
		if found {
			return "/" + target + " " + remainder
		}
		return "/" + target
	}
	return text
}

func parseRegisteredCommand(text string) (command.Request, command.Command, bool) {
	return builtinCommandRegistry.Parse(normalizeDotCommand(text))
}

// builtinCommandRegistry holds every command file in internal/inbound/commands.
// The inbound package owns the boundary concerns (resume, authorization, and
// wiring); what a command does lives in its own file.
var builtinCommandRegistry = mustCommandRegistry(builtincommands.All())

func mustCommandRegistry(commands []command.Command) *command.Registry {
	registry, err := command.NewRegistry(commands)
	if err != nil {
		// Commands are compiled in. A failure is a programmer error, never
		// user input or runtime configuration.
		panic(fmt.Sprintf("invalid built-in command registry: %v", err))
	}
	return registry
}

func CommandRegistry() *command.Registry { return builtinCommandRegistry }
