package llm

import (
	"context"
	"encoding/json"
	"time"
)

// CallStatus mirrors the stored llm_call status so a plan can be explained
// later: a live answer, a cache hit, or a failure.
type CallStatus string

const (
	StatusOK     CallStatus = "ok"
	StatusError  CallStatus = "error"
	StatusCached CallStatus = "cached"
)

// Request asks for one structured answer. Schema describes the only shape the
// provider may return, so Go can unmarshal it without parsing prose.
type Request struct {
	System      string
	Prompt      string
	ToolName    string
	Schema      json.RawMessage
	Temperature float64
	MaxTokens   int
	PlanID      *int64
}

type Response struct {
	Content   string
	Cached    bool
	TokensIn  int
	TokensOut int
	Latency   time.Duration
}

type CacheKey struct {
	PromptHash  string
	Model       string
	Temperature float64
}

type CacheEntry struct {
	PromptHash  string
	Model       string
	Temperature float64
	Response    string
}

// CallRecord is the audit row for one attempt at generating candidates.
type CallRecord struct {
	PlanID      *int64
	Provider    string
	Model       string
	Temperature float64
	PromptHash  string
	Prompt      string
	Response    string
	TokensIn    int
	TokensOut   int
	Latency     time.Duration
	Status      CallStatus
	Error       string
}

type CallStore interface {
	Lookup(ctx context.Context, key CacheKey) (response string, found bool, err error)
	SaveCache(ctx context.Context, entry CacheEntry) error
	LogCall(ctx context.Context, record CallRecord) error
}
