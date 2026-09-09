// Package llm stores the provider call audit trail and the prompt cache.
package llm

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	llmcore "github.com/ZetoOfficial/meal-planner/internal/llm"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

// Lookup serves a previous answer for the exact same prompt. A cache entry that
// was written for a different model is ignored rather than reused.
func (r *Repository) Lookup(ctx context.Context, key llmcore.CacheKey) (response string, found bool, err error) {
	row, err := r.store.Queries(ctx).LookupLLMCache(ctx, key.PromptHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, dberror.Map(err, "llm cache")
	}
	if row.Model != key.Model {
		return "", false, nil
	}
	return row.Response, true, nil
}

func (r *Repository) SaveCache(ctx context.Context, entry llmcore.CacheEntry) error {
	_, err := r.store.Queries(ctx).UpsertLLMCache(ctx, sqlc.UpsertLLMCacheParams{
		PromptHash:  entry.PromptHash,
		Model:       entry.Model,
		Temperature: pgvalue.Numeric(entry.Temperature),
		Response:    entry.Response,
	})
	if err != nil {
		return dberror.Map(err, "llm cache")
	}
	return nil
}

func (r *Repository) LogCall(ctx context.Context, record llmcore.CallRecord) error {
	_, err := r.store.Queries(ctx).CreateLLMCall(ctx, sqlc.CreateLLMCallParams{
		PlanID:      record.PlanID,
		Provider:    record.Provider,
		Model:       record.Model,
		Temperature: pgvalue.Numeric(record.Temperature),
		PromptHash:  record.PromptHash,
		Prompt:      record.Prompt,
		Response:    optionalText(record.Response),
		TokensIn:    optionalInt32(record.TokensIn),
		TokensOut:   optionalInt32(record.TokensOut),
		LatencyMs:   optionalInt32(int(record.Latency / time.Millisecond)),
		Status:      sqlc.LlmCallStatus(record.Status),
		Error:       optionalText(record.Error),
	})
	if err != nil {
		return dberror.Map(err, "llm call")
	}
	return nil
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalInt32(value int) *int32 {
	if value <= 0 || int64(value) > math.MaxInt32 {
		return nil
	}
	converted := int32(value)
	return &converted
}
