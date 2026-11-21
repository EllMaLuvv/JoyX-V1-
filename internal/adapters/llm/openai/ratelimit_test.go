package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Chomosuke9/WazzapAgent-Go/internal/agent"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/identity"
	"github.com/Chomosuke9/WazzapAgent-Go/internal/inbound"
)

// A throttled request must be replayed after the provider's own hint instead of
// failing the turn: Groq's quota window opens within seconds, and giving up makes
// every message in an active chat die.
func TestThrottledRequestIsRetriedAfterTheProviderHint(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"error":{"message":"Rate limit reached ... try again in 0.10s."}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"balasan"}}]}`))
	}))
	defer server.Close()

	providerID, err := identity.ParseProviderID("openai-compatible")
	if err != nil {
		t.Fatalf("parse provider: %v", err)
	}
	client, err := New(Config{
		Endpoint: server.URL, APIKey: "secret", ProviderID: providerID,
		SystemPolicy: "SAFETY", Timeout: 5 * time.Second, Concurrency: 1,
		MaxResponseBytes: 4096, Commands: inbound.CommandRegistry(), RateLimitRetries: 3,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	result, err := client.Generate(context.Background(), modelRequest(t, providerID))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.Text != "balasan" {
		t.Fatalf("text = %q", result.Text)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

// With retries switched off the client keeps its old contract: one attempt and a
// typed rate-limit error.
func TestThrottleRetryIsOptIn(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"message":"Rate limit reached"}}`))
	}))
	defer server.Close()

	providerID, _ := identity.ParseProviderID("openai-compatible")
	client, err := New(Config{
		Endpoint: server.URL, APIKey: "secret", ProviderID: providerID,
		SystemPolicy: "SAFETY", Timeout: 5 * time.Second, Concurrency: 1,
		MaxResponseBytes: 4096, Commands: inbound.CommandRegistry(),
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	_, err = client.Generate(context.Background(), modelRequest(t, providerID))
	if !agent.IsCode(err, agent.ErrorRateLimited) {
		t.Fatalf("err = %v, want rate_limited", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}
