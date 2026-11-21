package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/inbound"
)

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func testStoreKey(t *testing.T) agent.Key {
	t.Helper()
	tenantID, err := identity.NewTenantID()
	if err != nil {
		t.Fatal(err)
	}
	accountID, err := identity.NewAccountID()
	if err != nil {
		t.Fatal(err)
	}
	chatID, err := identity.NewChatID()
	if err != nil {
		t.Fatal(err)
	}
	return agent.Key{TenantID: tenantID, AccountID: accountID, ChatID: chatID}
}

func TestDeadlineStoreSavesListsAndMarksReminded(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	key := testStoreKey(t)
	id, err := identity.NewCausationID()
	if err != nil {
		t.Fatal(err)
	}
	source, err := identity.NewMessageID()
	if err != nil {
		t.Fatal(err)
	}
	deadline := inbound.Deadline{Key: key, ID: id, Source: source, Name: "Makalah", Deadline: time.Date(2026, 10, 7, 23, 59, 0, 0, time.UTC)}
	created, err := store.Inbound().SaveDeadline(ctx, deadline)
	if err != nil || !created {
		t.Fatalf("save = %v, %v", created, err)
	}
	again, err := store.Inbound().SaveDeadline(ctx, deadline)
	if err != nil || again {
		t.Fatalf("a second save from the same message = %v, %v", again, err)
	}

	listed, err := store.Inbound().ListDeadlines(ctx, key.TenantID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list = %v, %v", listed, err)
	}
	if listed[0].Name != "Makalah" || listed[0].Reminded {
		t.Fatalf("listed = %+v", listed[0])
	}

	if err := store.Inbound().MarkDeadlineReminded(ctx, id); err != nil {
		t.Fatalf("mark reminded: %v", err)
	}
	listed, err = store.Inbound().ListDeadlines(ctx, key.TenantID)
	if err != nil || len(listed) != 1 || !listed[0].Reminded {
		t.Fatalf("listed after reminder = %+v, %v", listed, err)
	}

	if err := store.Inbound().DeleteDeadline(ctx, key, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	listed, err = store.Inbound().ListDeadlines(ctx, key.TenantID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("listed after delete = %+v, %v", listed, err)
	}
}

func TestSentMessageStoreRecordsAndDeletes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	key := testStoreKey(t)
	if err := store.RecordSentMessage(ctx, key, "3EB0ABC"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := store.RecordSentMessage(ctx, key, "3EB0DEF"); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Recording the same message twice is not an error and does not duplicate.
	if err := store.RecordSentMessage(ctx, key, "3EB0ABC"); err != nil {
		t.Fatalf("record again: %v", err)
	}

	listed, err := store.SentMessages(ctx, key)
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed = %q, %v", listed, err)
	}
	if !containsString(listed, "3EB0ABC") || !containsString(listed, "3EB0DEF") {
		t.Fatalf("listed = %q", listed)
	}

	if err := store.DeleteSentMessage(ctx, key, "3EB0ABC"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	listed, err = store.SentMessages(ctx, key)
	if err != nil || len(listed) != 1 || listed[0] != "3EB0DEF" {
		t.Fatalf("listed after delete = %q, %v", listed, err)
	}
}
