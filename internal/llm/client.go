// Package llm is the provider adapter: it turns a prompt plus an output schema
// into structured text, caches identical prompts and logs every call. It never
// decides anything about a plan; Go validates and the solver chooses.
package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

type Client struct {
	config   Config
	provider Provider
	store    CallStore
	http     *http.Client
}

// New builds a client on the supplied wire format. The provider name wins over
// Config.Provider so audit rows cannot claim a provider that never answered.
func New(config Config, store CallStore, provider Provider) (*Client, error) {
	config = config.withDefaults()
	config.Provider = provider.Name()
	if err := config.Validate(); err != nil {
		return nil, apperr.Validation(err.Error())
	}
	if config.Endpoint == "" {
		config.Endpoint = provider.DefaultEndpoint()
	}
	return &Client{
		config:   config,
		provider: provider,
		store:    store,
		http:     &http.Client{Timeout: config.Timeout},
	}, nil
}

// Generate answers from the cache when the exact prompt was seen before and
// otherwise calls the provider, retrying only transient failures.
func (c *Client) Generate(ctx context.Context, request Request) (Response, error) {
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
