package inbound

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/action"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// ReminderLead is how long before its deadline a task's reminder goes out.
const ReminderLead = 24 * time.Hour

// MaxDeadlinesPerChat bounds one chat's deadline list.
const MaxDeadlinesPerChat = 50

// deadlineCodeLength is how many trailing characters of a deadline's ID the
// /dd list shows and people type to delete one.
const deadlineCodeLength = 6

// deadlineCheckInterval is how often due reminders are looked for. A deadline
// is stored, not armed as a timer, so a reminder survives a restart and a late
// one still goes out.
const deadlineCheckInterval = 30 * time.Second

// Deadline is one stored task deadline.
type Deadline struct {
	Key      agent.Key
	ID       identity.CausationID
	Source   identity.MessageID
	Name     string
	Deadline time.Time
	Reminded bool
}

// Validate reports whether the deadline is complete and storable.
func (deadline Deadline) Validate() error {
	if err := deadline.Key.Validate(); err != nil {
		return err
	}
	if deadline.ID.IsZero() || deadline.Source.IsZero() || deadline.Deadline.IsZero() {
		return errors.New("deadline ID, source message and deadline time are required")
	}
	if strings.TrimSpace(deadline.Name) == "" || !utf8.ValidString(deadline.Name) || len(deadline.Name) > 200 {
		return errors.New("name must be non-empty valid UTF-8 within 200 bytes")
	}
	return nil
}

// Code is the short ID /dd list shows and people type to delete the deadline.
func (deadline Deadline) Code() string {
	id := deadline.ID.String()
	return id[max(len(id)-deadlineCodeLength, 0):]
}

// RemindAt is when the reminder is due: the day before the deadline.
func (deadline Deadline) RemindAt() time.Time { return deadline.Deadline.Add(-ReminderLead) }

// DeadlineStore keeps deadline reminders.
type DeadlineStore interface {
	// SaveDeadline reports whether the deadline is new; a second deadline from
	// the same source message is not stored.
	SaveDeadline(context.Context, Deadline) (bool, error)
	ListDeadlines(context.Context, identity.TenantID) ([]Deadline, error)
	DeleteDeadline(context.Context, agent.Key, identity.CausationID) error
	// MarkDeadlineReminded records that the reminder went out. A failure is
	// reported and the reminder may be sent again; the text is a reminder, not
	// a one-time secret.
	MarkDeadlineReminded(context.Context, identity.CausationID) error
}

// AddDeadline saves a reminder for name due at deadline, and reports whether it
// is new. It implements command.DeadlineEditor for /dd.
func (dispatcher *Dispatcher) AddDeadline(ctx context.Context, key agent.Key, source identity.MessageID, name string, deadline time.Time) (command.Deadline, error) {
	if err := key.Validate(); err != nil {
		return command.Deadline{}, err
	}
	if source.IsZero() {
		return command.Deadline{}, agent.NewError(agent.ErrorInvalidArgument, "add deadline", errors.New("source message is required"))
	}
	if deadline.IsZero() {
		return command.Deadline{}, agent.NewError(agent.ErrorInvalidArgument, "add deadline", errors.New("deadline time is required"))
	}
	id, err := identity.NewCausationID()
	if err != nil {
		return command.Deadline{}, agent.NewError(agent.ErrorInternal, "add deadline", err)
	}
	stored := Deadline{Key: key, ID: id, Source: source, Name: name, Deadline: deadline.UTC()}
	if err := stored.Validate(); err != nil {
		return command.Deadline{}, agent.NewError(agent.ErrorInvalidArgument, "add deadline", err)
	}
	created, err := dispatcher.store.SaveDeadline(ctx, stored)
	if err != nil {
		return command.Deadline{}, err
	}
	if !created {
		// The same message already holds a deadline; hand back the stored one so
		// a replayed command does not look like a new task.
		return dispatcher.findDeadline(ctx, key, source)
	}
	if err := dispatcher.enforceDeadlineLimit(ctx, key); err != nil {
		return command.Deadline{}, err
	}
	return dispatcher.commandDeadline(stored), nil
}

// Deadlines lists one chat's deadlines, soonest first. It implements
// command.DeadlineEditor for /dd.
func (dispatcher *Dispatcher) Deadlines(ctx context.Context, key agent.Key) ([]command.Deadline, error) {
	deadlines, err := dispatcher.chatDeadlines(ctx, key)
	if err != nil {
		return nil, err
	}
	listed := make([]command.Deadline, 0, len(deadlines))
	for _, deadline := range deadlines {
		listed = append(listed, dispatcher.commandDeadline(deadline))
	}
	return listed, nil
}

// DeleteDeadline removes one of the chat's deadlines by its short code. It
// reports false when there is no such deadline. It implements
// command.DeadlineEditor for /dd.
func (dispatcher *Dispatcher) DeleteDeadline(ctx context.Context, key agent.Key, code string) (bool, error) {
	deadlines, err := dispatcher.chatDeadlines(ctx, key)
	if err != nil {
		return false, err
	}
	for _, deadline := range deadlines {
		if !strings.EqualFold(deadline.Code(), strings.TrimSpace(code)) {
			continue
		}
		if err := dispatcher.store.DeleteDeadline(ctx, key, deadline.ID); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (dispatcher *Dispatcher) commandDeadline(deadline Deadline) command.Deadline {
	return command.Deadline{
		Code:     deadline.Code(),
		Name:     deadline.Name,
		Deadline: deadline.Deadline.In(dispatcher.options.Location),
		Reminded: deadline.Reminded,
	}
}

func (dispatcher *Dispatcher) chatDeadlines(ctx context.Context, key agent.Key) ([]Deadline, error) {
	deadlines, err := dispatcher.store.ListDeadlines(ctx, key.TenantID)
	if err != nil {
		return nil, err
	}
	chat := deadlines[:0]
	for _, deadline := range deadlines {
		if deadline.Key == key {
			chat = append(chat, deadline)
		}
	}
	return chat, nil
}

// findDeadline returns the deadline a source message already holds.
func (dispatcher *Dispatcher) findDeadline(ctx context.Context, key agent.Key, source identity.MessageID) (command.Deadline, error) {
	deadlines, err := dispatcher.chatDeadlines(ctx, key)
	if err != nil {
		return command.Deadline{}, err
	}
	for _, deadline := range deadlines {
		if deadline.Source == source {
			return dispatcher.commandDeadline(deadline), nil
		}
	}
	return command.Deadline{}, agent.NewError(agent.ErrorNotFound, "add deadline", errors.New("the deadline this message holds is missing"))
}

// enforceDeadlineLimit drops the oldest reminded deadlines of a chat that has
// more than MaxDeadlinesPerChat, so an unbounded list cannot grow forever.
func (dispatcher *Dispatcher) enforceDeadlineLimit(ctx context.Context, key agent.Key) error {
	deadlines, err := dispatcher.chatDeadlines(ctx, key)
	if err != nil {
		return err
	}
	if len(deadlines) <= MaxDeadlinesPerChat {
		return nil
	}
	reminded := make([]Deadline, 0, len(deadlines))
	for _, deadline := range deadlines {
		if deadline.Reminded {
			reminded = append(reminded, deadline)
		}
	}
	reminded = sortDeadlinesByDeadline(reminded)
	for len(deadlines) > MaxDeadlinesPerChat && len(reminded) > 0 {
		if err := dispatcher.store.DeleteDeadline(ctx, key, reminded[0].ID); err != nil {
			return err
		}
		reminded = reminded[1:]
		deadlines = deadlines[:len(deadlines)-1]
	}
	return nil
}

// sortDeadlinesByDeadline orders deadlines soonest first.
func sortDeadlinesByDeadline(deadlines []Deadline) []Deadline {
	for i := 1; i < len(deadlines); i++ {
		for j := i; j > 0 && deadlines[j].Deadline.Before(deadlines[j-1].Deadline); j-- {
			deadlines[j], deadlines[j-1] = deadlines[j-1], deadlines[j]
		}
	}
	return deadlines
}

// ReminderLoop sends the reminders that have come due. It runs beside the
// dispatcher for the whole process, and stops with ctx.
func (dispatcher *Dispatcher) ReminderLoop(ctx context.Context, tenantID identity.TenantID) error {
	ticker := time.NewTicker(deadlineCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			dispatcher.sendDueDeadlineReminders(ctx, tenantID)
		}
	}
}

// sendDueDeadlineReminders sends one reminder for every stored deadline whose
// day-before moment has passed and that has not been reminded yet.
func (dispatcher *Dispatcher) sendDueDeadlineReminders(ctx context.Context, tenantID identity.TenantID) {
	if dispatcher.platform.Text == nil {
		return
	}
	deadlines, err := dispatcher.store.ListDeadlines(ctx, tenantID)
	if err != nil {
		dispatcher.options.Report(err)
		return
	}
	now := dispatcher.options.Clock.Now()
	for _, deadline := range deadlines {
		if deadline.Reminded || now.Before(deadline.RemindAt()) {
			continue
		}
		if err := dispatcher.sendDeadlineReminder(ctx, deadline); err != nil {
			if agent.IsCode(err, agent.ErrorNotReady) {
				return // the account is offline; the next tick tries again
			}
			dispatcher.options.Report(err)
			continue
		}
		if err := dispatcher.store.MarkDeadlineReminded(ctx, deadline.ID); err != nil {
			dispatcher.options.Report(err)
		}
	}
}

func (dispatcher *Dispatcher) sendDeadlineReminder(ctx context.Context, deadline Deadline) error {
	actionID, err := identity.NewActionID()
	if err != nil {
		return agent.NewError(agent.ErrorInternal, "send deadline reminder", err)
	}
	text := fmt.Sprintf("DEADLINE %s Adalah Tanggal %s", deadline.Name, formatDeadlineDate(deadline.Deadline.In(dispatcher.options.Location)))
	_, err = dispatcher.platform.Text.SendText(ctx, action.SendTextRequest{Key: deadline.Key, ActionID: actionID, Text: text})
	return err
}

// indonesianMonths are the month names Joy's reminders used.
var indonesianMonths = [12]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// formatDeadlineDate renders a date the way Joy did: "7 Oktober 2026".
func formatDeadlineDate(value time.Time) string {
	return fmt.Sprintf("%d %s %d", value.Day(), indonesianMonths[int(value.Month())-1], value.Year())
}
