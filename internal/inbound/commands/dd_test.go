package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/command"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/conversation"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
)

// recordingDeadlines stands in for the dispatcher's deadline store.
type recordingDeadlines struct {
	added   []command.Deadline
	listed  []command.Deadline
	deleted []string
	fail    error
}

func (recorder *recordingDeadlines) AddDeadline(_ context.Context, _ agent.Key, _ identity.MessageID, name string, deadline time.Time) (command.Deadline, error) {
	if recorder.fail != nil {
		return command.Deadline{}, recorder.fail
	}
	value := command.Deadline{Code: "a1b2c3", Name: name, Deadline: deadline}
	recorder.added = append(recorder.added, value)
	return value, nil
}

func (recorder *recordingDeadlines) Deadlines(context.Context, agent.Key) ([]command.Deadline, error) {
	return recorder.listed, nil
}

func (recorder *recordingDeadlines) DeleteDeadline(_ context.Context, _ agent.Key, code string) (bool, error) {
	recorder.deleted = append(recorder.deleted, code)
	return true, nil
}

func runDeadlineCommand(t *testing.T, text string, deadlines *recordingDeadlines) (*recordingText, error) {
	t.Helper()
	registry := builtinRegistry(t)
	request, _, recognized := registry.Parse(text)
	if !recognized {
		t.Fatalf("%q is not a registered command", text)
	}
	key := testChatKey(t)
	messageID, _ := identity.NewMessageID()
	replies := &recordingText{}
	err := registry.Dispatch(context.Background(), request, command.Invocation{
		Message:  conversation.IncomingMessage{TenantID: key.TenantID, AccountID: key.AccountID, ChatID: key.ChatID, ID: messageID, Text: text},
		Facts:    command.PermissionFacts{IsPrivate: true},
		Platform: command.Platform{Text: replies, Deadlines: deadlines},
		Store:    &handledStore{},
	})
	return replies, err
}

func TestParseDeadlineArgs(t *testing.T) {
	cases := []struct {
		args    string
		name    string
		day     int
		month   time.Month
		year    int
		wantErr bool
	}{
		{args: "Makalah Basis Data 07-10-2026", name: "Makalah Basis Data", day: 7, month: time.October, year: 2026},
		{args: "Tugas 1/11/2026", name: "Tugas", day: 1, month: time.November, year: 2026},
		{args: "Ujian 2026-12-31", name: "Ujian", day: 31, month: time.December, year: 2026},
		{args: "Laporan 7-10-26", name: "Laporan", day: 7, month: time.October, year: 2026},
		{args: "TanpaTanggal", wantErr: true},
		{args: "32-13-2026", wantErr: true},
		{args: "07-10-2026", wantErr: true},
	}
	for _, testCase := range cases {
		name, at, err := parseDeadlineArgs(testCase.args)
		if testCase.wantErr {
			if err == nil {
				t.Fatalf("parseDeadlineArgs(%q) = %q, %v; want an error", testCase.args, name, at)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseDeadlineArgs(%q): %v", testCase.args, err)
		}
		if name != testCase.name || at.Day() != testCase.day || at.Month() != testCase.month || at.Year() != testCase.year {
			t.Fatalf("parseDeadlineArgs(%q) = %q, %v; want %q, %02d-%02d-%04d", testCase.args, name, at, testCase.name, testCase.day, testCase.month, testCase.year)
		}
	}
}

func TestDeadlineCommandAddsAndLists(t *testing.T) {
	deadlines := &recordingDeadlines{}
	replies, err := runDeadlineCommand(t, "/dd Makalah Basis Data 07-10-2026", deadlines)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(deadlines.added) != 1 || deadlines.added[0].Name != "Makalah Basis Data" {
		t.Fatalf("added = %+v", deadlines.added)
	}
	if len(replies.sent) != 1 {
		t.Fatalf("sent = %q", replies.sent)
	}

	deadlines.listed = []command.Deadline{{Code: "a1b2c3", Name: "Makalah", Deadline: time.Date(2026, 10, 7, 23, 59, 0, 0, time.Local)}}
	replies, err = runDeadlineCommand(t, "/dd", deadlines)
	if err != nil {
		t.Fatalf("dispatch list: %v", err)
	}
	if len(replies.sent) != 1 || !strings.Contains(replies.sent[0], "Makalah") || !strings.Contains(replies.sent[0], "07-10-2026") {
		t.Fatalf("list reply = %q", replies.sent)
	}
}

func TestDeadlineCommandDeletes(t *testing.T) {
	deadlines := &recordingDeadlines{}
	replies, err := runDeadlineCommand(t, "/dd del a1b2c3", deadlines)
	if err != nil {
		t.Fatalf("dispatch delete: %v", err)
	}
	if len(deadlines.deleted) != 1 || deadlines.deleted[0] != "a1b2c3" {
		t.Fatalf("deleted = %q", deadlines.deleted)
	}
	if len(replies.sent) != 1 {
		t.Fatalf("sent = %q", replies.sent)
	}
}

func TestDeadlineCommandReportsStoreFailure(t *testing.T) {
	deadlines := &recordingDeadlines{fail: errors.New("database is locked")}
	if _, err := runDeadlineCommand(t, "/dd Makalah 07-10-2026", deadlines); err == nil {
		t.Fatal("a store failure must reach the caller")
	}
}
