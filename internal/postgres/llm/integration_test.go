//go:build integration

package llm_test

import (
	"context"
	"testing"
	"time"

	llmcore "github.com/ZetoOfficial/meal-planner/internal/llm"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	llmrepo "github.com/ZetoOfficial/meal-planner/internal/postgres/llm"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
)

func TestLLMCacheRoundTripAndModelIsolation(t *testing.T) {
	repository, ctx := newRepository(t)
	entry := llmcore.CacheEntry{
		PromptHash:  "hash-1",
		Model:       "claude-sonnet-5",
		Temperature: 0.2,
		Response:    `{"name":"Chicken bowl"}`,
	}

	if err := repository.SaveCache(ctx, entry); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	response, found, err := repository.Lookup(ctx, llmcore.CacheKey{
		PromptHash: entry.PromptHash, Model: entry.Model, Temperature: entry.Temperature,
	})
	if err != nil || !found || response != entry.Response {
		t.Fatalf("Lookup() = (%q, %t, %v), want the stored response", response, found, err)
	}

	_, found, err = repository.Lookup(ctx, llmcore.CacheKey{
		PromptHash: entry.PromptHash, Model: "other-model", Temperature: entry.Temperature,
	})
	if err != nil || found {
		t.Fatalf("Lookup(other model) = (%t, %v), want a miss so a model switch cannot reuse an answer", found, err)
	}
}

func TestLLMCallsAreLoggedWithStatusAndUsage(t *testing.T) {
	repository, ctx := newRepository(t)

	for _, record := range []llmcore.CallRecord{
		{
			Provider: "anthropic", Model: "claude-sonnet-5", Temperature: 0.2,
			PromptHash: "hash-ok", Prompt: "generate", Response: `{"name":"ok"}`,
			TokensIn: 120, TokensOut: 45, Latency: 1500 * time.Millisecond, Status: llmcore.StatusOK,
		},
		{
			Provider: "anthropic", Model: "claude-sonnet-5", Temperature: 0.2,
			PromptHash: "hash-fail", Prompt: "generate", Status: llmcore.StatusError,
			Error: "provider unavailable",
		},
		{
			Provider: "anthropic", Model: "claude-sonnet-5", Temperature: 0.2,
			PromptHash: "hash-ok", Prompt: "generate", Response: `{"name":"ok"}`,
			Status: llmcore.StatusCached,
		},
	} {
		if err := repository.LogCall(ctx, record); err != nil {
			t.Fatalf("LogCall(%s) error = %v", record.Status, err)
		}
	}

	rows, err := store.Queries(ctx).ListLLMCalls(ctx, 10)
	if err != nil {
		t.Fatalf("ListLLMCalls() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("logged calls = %d, want live, failed and cached calls all recorded", len(rows))
	}

	statuses := map[string]bool{}
	for _, row := range rows {
		statuses[string(row.Status)] = true
		if row.Status == "ok" {
			if row.TokensIn == nil || *row.TokensIn != 120 || row.LatencyMs == nil || *row.LatencyMs != 1500 {
				t.Fatalf("ok call = %#v, want usage and latency recorded", row)
			}
		}
		if row.Status == "error" && (row.Error == nil || *row.Error == "") {
			t.Fatalf("error call = %#v, want the failure reason recorded", row)
		}
	}
	for _, status := range []string{"ok", "error", "cached"} {
		if !statuses[status] {
			t.Fatalf("statuses = %#v, want %q logged", statuses, status)
		}
	}
}

func newRepository(t *testing.T) (*llmrepo.Repository, context.Context) {
	t.Helper()
	database := testpostgres.RequireDatabase(t)
	t.Cleanup(func() { store = nil })
	store = postgresdb.NewStore(database.Pool)
	return llmrepo.New(store), context.Background()
}

var store *postgresdb.Store
