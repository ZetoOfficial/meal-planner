package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

func TestGenerateReturnsStructuredContentAndLogsTheCall(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeToolUse(t, w, `{"name":"Chicken bowl"}`, 120, 45)
	}))
	defer server.Close()

	store := &fakeStore{}
	client := newTestClient(t, server.URL, store)

	response, err := client.Generate(t.Context(), testRequest())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if response.Content != `{"name":"Chicken bowl"}` {
		t.Fatalf("content = %q, want the tool arguments", response.Content)
	}
	if response.Cached {
		t.Fatal("response reported as cached, want a live call")
	}
	if body["model"] != "claude-sonnet-5" || body["temperature"] != 0.2 {
		t.Fatalf("request body = %#v, want the configured model and temperature", body)
	}
	if len(store.calls) != 1 {
		t.Fatalf("logged calls = %d, want one", len(store.calls))
	}
	call := store.calls[0]
	if call.Status != StatusOK || call.TokensIn != 120 || call.TokensOut != 45 {
		t.Fatalf("logged call = %#v, want an ok call with token counts", call)
	}
	if call.PromptHash == "" || call.Prompt == "" || call.Response == "" {
		t.Fatalf("logged call = %#v, want the prompt and response recorded", call)
	}
	if len(store.saved) != 1 || store.saved[0].PromptHash != call.PromptHash {
		t.Fatalf("cached entries = %#v, want the response cached under its prompt hash", store.saved)
	}
}

func TestGenerateServesCachedResponsesWithoutCallingTheProvider(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeToolUse(t, w, `{"name":"live"}`, 1, 1)
	}))
	defer server.Close()

	store := &fakeStore{cached: `{"name":"cached"}`}
	client := newTestClient(t, server.URL, store)

	response, err := client.Generate(t.Context(), testRequest())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if response.Content != `{"name":"cached"}` || !response.Cached {
		t.Fatalf("response = %#v, want the cached content", response)
	}
	if requests.Load() != 0 {
		t.Fatalf("provider requests = %d, want none for a cache hit", requests.Load())
	}
	if len(store.calls) != 1 || store.calls[0].Status != StatusCached {
		t.Fatalf("logged calls = %#v, want one cached call logged", store.calls)
	}
}

func TestGenerateRetriesTransientProviderFailures(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeToolUse(t, w, `{"name":"third try"}`, 10, 5)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, &fakeStore{})

	response, err := client.Generate(t.Context(), testRequest())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Content != `{"name":"third try"}` {
		t.Fatalf("content = %q, want the successful retry", response.Content)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want two retries before success", attempts.Load())
	}
}

func TestGenerateReportsUnavailableAfterExhaustingRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(529)
	}))
	defer server.Close()

	store := &fakeStore{}
	client := newTestClient(t, server.URL, store)

	_, err := client.Generate(t.Context(), testRequest())

	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != apperr.CodeUnavailable {
		t.Fatalf("Generate() error = %v, want an unavailable application error", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want the configured attempt budget", attempts.Load())
	}
	if len(store.calls) != 1 || store.calls[0].Status != StatusError || store.calls[0].Error == "" {
		t.Fatalf("logged calls = %#v, want one failed call with its error", store.calls)
	}
}

func TestGenerateDoesNotRetryClientErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, &fakeStore{})

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a provider error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want no retry for a permanent client error", attempts.Load())
	}
}

func TestGenerateFailsWhenTheProviderReturnsNoToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"sorry"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	store := &fakeStore{}
	client := newTestClient(t, server.URL, store)

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want an error for a response without structured output")
	}
	if len(store.calls) != 1 || store.calls[0].Status != StatusError {
		t.Fatalf("logged calls = %#v, want the unusable response logged as an error", store.calls)
	}
}

func TestGenerateHonoursTheConfiguredTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		writeToolUse(t, w, `{}`, 1, 1)
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	client := New(Config{
		Provider:    "anthropic",
		Model:       "claude-sonnet-5",
		Endpoint:    server.URL,
		APIKey:      "test-key",
		Timeout:     20 * time.Millisecond,
		MaxAttempts: 1,
		BaseBackoff: time.Millisecond,
	}, &fakeStore{})

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a timeout error")
	}
}

func TestNewRejectsIncompleteConfiguration(t *testing.T) {
	cases := map[string]Config{
		"no model":    {Endpoint: "http://localhost", APIKey: "k"},
		"no endpoint": {Model: "m", APIKey: "k"},
		"no api key":  {Model: "m", Endpoint: "http://localhost"},
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			if err := config.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", name)
			}
		})
	}
}

func newTestClient(t *testing.T, endpoint string, store CallStore) *Client {
	t.Helper()
	return New(Config{
		Provider:    "anthropic",
		Model:       "claude-sonnet-5",
		Endpoint:    endpoint,
		APIKey:      "test-key",
		Timeout:     2 * time.Second,
		MaxAttempts: 3,
		BaseBackoff: time.Millisecond,
	}, store)
}

func testRequest() Request {
	return Request{
		System:      "You generate recipes.",
		Prompt:      "Generate one recipe using chicken.",
		ToolName:    "emit_recipe",
		Schema:      json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
		Temperature: 0.2,
	}
}

func writeToolUse(t *testing.T, w http.ResponseWriter, arguments string, tokensIn, tokensOut int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	payload := map[string]any{
		"content": []map[string]any{
			{"type": "tool_use", "name": "emit_recipe", "input": json.RawMessage(arguments)},
		},
		"usage": map[string]int{"input_tokens": tokensIn, "output_tokens": tokensOut},
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("write provider response: %v", err)
	}
}

type fakeStore struct {
	cached string
	calls  []CallRecord
	saved  []CacheEntry
}

func (s *fakeStore) Lookup(_ context.Context, _ CacheKey) (response string, found bool, err error) {
	if s.cached == "" {
		return "", false, nil
	}
	return s.cached, true, nil
}

func (s *fakeStore) SaveCache(_ context.Context, entry CacheEntry) error {
	s.saved = append(s.saved, entry)
	return nil
}

func (s *fakeStore) LogCall(_ context.Context, record CallRecord) error {
	s.calls = append(s.calls, record)
	return nil
}
