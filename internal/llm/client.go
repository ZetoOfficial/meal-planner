// Package llm is the provider adapter: it turns a prompt plus an output schema
// into structured text, caches identical prompts and logs every call. It never
// decides anything about a plan; Go validates and the solver chooses.
package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxAttempts = 3
	defaultBaseBackoff = 500 * time.Millisecond
	maxBackoff         = 8 * time.Second
)

// CallStatus mirrors the stored llm_call status so a plan can be explained
// later: a live answer, a cache hit, or a failure.
type CallStatus string

const (
	StatusOK     CallStatus = "ok"
	StatusError  CallStatus = "error"
	StatusCached CallStatus = "cached"
)

type Config struct {
	Provider    string
	Model       string
	Endpoint    string
	APIKey      string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
}

func (c Config) Validate() error {
	if c.Model == "" || c.Endpoint == "" || c.APIKey == "" {
		return errors.New("llm config requires a model, an endpoint and an api key")
	}
	return nil
}

func (c Config) withDefaults() Config {
	if c.Provider == "" {
		c.Provider = "anthropic"
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = defaultBaseBackoff
	}
	return c
}

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

type Clock interface {
	Now() time.Time
}

type Client struct {
	config Config
	store  CallStore
	http   *http.Client
}

func New(config Config, store CallStore) *Client {
	config = config.withDefaults()
	return &Client{
		config: config,
		store:  store,
		http:   &http.Client{Timeout: config.Timeout},
	}
}

// Generate answers from the cache when the exact prompt was seen before and
// otherwise calls the provider, retrying only transient failures.
func (c *Client) Generate(ctx context.Context, request Request) (Response, error) {
	if err := c.config.Validate(); err != nil {
		return Response{}, apperr.Validation(err.Error())
	}
	hash := c.promptHash(request)

	if cached, found, err := c.store.Lookup(ctx, CacheKey{
		PromptHash:  hash,
		Model:       c.config.Model,
		Temperature: request.Temperature,
	}); err == nil && found {
		c.log(ctx, c.record(request, hash, StatusCached, cached, 0, 0, 0, ""))
		return Response{Content: cached, Cached: true}, nil
	}

	response, err := c.callProvider(ctx, request)
	if err != nil {
		c.log(ctx, c.record(request, hash, StatusError, "", 0, 0, response.Latency, err.Error()))
		return Response{}, err
	}

	c.log(ctx, c.record(request, hash, StatusOK, response.Content,
		response.TokensIn, response.TokensOut, response.Latency, ""))
	// A cache write that fails must not discard a valid candidate; the next
	// identical prompt simply costs another provider call.
	_ = c.store.SaveCache(ctx, CacheEntry{
		PromptHash:  hash,
		Model:       c.config.Model,
		Temperature: request.Temperature,
		Response:    response.Content,
	})
	return response, nil
}

// callProvider retries transient provider failures with bounded exponential
// backoff and gives up with a 503-mappable error.
func (c *Client) callProvider(ctx context.Context, request Request) (Response, error) {
	var lastErr error
	backoff := c.config.BaseBackoff
	for attempt := range c.config.MaxAttempts {
		if attempt > 0 {
			if err := sleep(ctx, backoff); err != nil {
				return Response{}, err
			}
			backoff = min(backoff*2, maxBackoff)
		}

		response, err := c.complete(ctx, request)
		if err == nil {
			return response, nil
		}
		lastErr = err

		var transient *transientError
		if !errors.As(err, &transient) {
			return Response{}, err
		}
	}
	return Response{}, apperr.Unavailable("recipe generation is unavailable, retry later or plan reuse-only", lastErr)
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) record(
	request Request,
	hash string,
	status CallStatus,
	response string,
	tokensIn, tokensOut int,
	latency time.Duration,
	failure string,
) CallRecord {
	return CallRecord{
		PlanID:      request.PlanID,
		Provider:    c.config.Provider,
		Model:       c.config.Model,
		Temperature: request.Temperature,
		PromptHash:  hash,
		Prompt:      request.Prompt,
		Response:    response,
		TokensIn:    tokensIn,
		TokensOut:   tokensOut,
		Latency:     latency,
		Status:      status,
		Error:       failure,
	}
}

// log never fails a generation: an unwritten audit row must not lose a valid
// recipe candidate.
func (c *Client) log(ctx context.Context, record CallRecord) {
	_ = c.store.LogCall(ctx, record)
}

// promptHash keys the cache on everything that changes the answer, so a model
// or temperature switch cannot serve a stale response.
func (c *Client) promptHash(request Request) string {
	digest := sha256.New()
	for _, part := range []string{
		c.config.Provider,
		c.config.Model,
		strconv.FormatFloat(request.Temperature, 'f', 2, 64),
		request.System,
		request.Prompt,
		request.ToolName,
		string(request.Schema),
	} {
		digest.Write([]byte(part))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

type transientError struct {
	status int
}

func (e *transientError) Error() string {
	return fmt.Sprintf("provider returned transient status %d", e.status)
}
