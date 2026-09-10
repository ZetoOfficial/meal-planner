package llm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
)

func TestGenerateReturnsStructuredContentAndLogsTheCall(t *testing.T) {
	provider := newStubProvider(Response{Content: `{"name":"Chicken bowl"}`, TokensIn: 120, TokensOut: 45})
	var body map[string]any
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		authorization = r.Header.Get("Authorization")
		writeStubPayload(w)
	}))
	defer server.Close()

	store := &fakeStore{}
	client := newTestClient(t, provider, server.URL, store)

	response, err := client.Generate(t.Context(), testRequest())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Proves Client.complete hands its own Config to the provider, not a zero
	// value and not the request: the stub echoes config.Model into the wire
	// body, so a client that passed Config{} (or the wrong config) here would
	// fail this check even though every other assertion in this test stays
	// green.
	if body["model"] != testModel {
		t.Fatalf("model = %v, want the client's configured model %q", body["model"], testModel)
	}
	// Same proof for Authorize: the stub echoes config.APIKey into the header,
	// so a client that passed the wrong Config there would fail this check too.
	if authorization != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want the client's configured API key", authorization)
	}
	if response.Content != `{"name":"Chicken bowl"}` {
		t.Fatalf("content = %q, want what the provider decoded", response.Content)
	}
	if response.Cached {
		t.Fatal("response reported as cached, want a live call")
	}
	if len(store.calls) != 1 {
		t.Fatalf("logged calls = %d, want one", len(store.calls))
	}
	call := store.calls[0]
	if call.Status != StatusOK || call.TokensIn != 120 || call.TokensOut != 45 {
		t.Fatalf("logged call = %#v, want an ok call with token counts", call)
	}
	if call.Provider != provider.Name() {
		t.Fatalf("logged provider = %q, want the provider that answered", call.Provider)
	}
	if call.PromptHash == "" || call.Prompt == "" || call.Response == "" {
		t.Fatalf("logged call = %#v, want the prompt and response recorded", call)
	}
	if len(store.saved) != 1 || store.saved[0].PromptHash != call.PromptHash {
		t.Fatalf("cached entries = %#v, want the response cached under its prompt hash", store.saved)
	}
}

// The default max-tokens budget used to be applied inside each provider's
// EncodeRequest, via a now-removed requestMaxTokens helper; the split moved that
// normalization into Client.complete, which makes it core behaviour and the core
// the place to prove it. Driving an unset budget end to end catches a regression
// there (an accidental deletion, or an off-by-one on the <= 0 check) instead of
// shipping it silently.
func TestGenerateDefaultsMaxTokensWhenUnset(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeStubPayload(w)
	}))
	defer server.Close()

	client := newTestClient(t, newStubProvider(Response{Content: "{}"}), server.URL, &fakeStore{})

	request := testRequest()
	request.MaxTokens = 0

	if _, err := client.Generate(t.Context(), request); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	got, _ := body["max_tokens"].(float64)
	if int(got) != defaultMaxTokens {
		t.Fatalf("max_tokens = %v, want the default budget %d applied for an unset request",
			body["max_tokens"], defaultMaxTokens)
	}
}

// The <= 0 guard in Client.complete must leave an already-set budget alone:
// only the caller knows why it chose a particular ceiling. This drives a
// non-zero MaxTokens through the real Generate path (the provider tests that
// assert 1024 call EncodeRequest directly and never reach the guard), so an
// unconditional overwrite of request.MaxTokens would fail it.
func TestGenerateLeavesAnExplicitMaxTokensUnchanged(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeStubPayload(w)
	}))
	defer server.Close()

	client := newTestClient(t, newStubProvider(Response{Content: "{}"}), server.URL, &fakeStore{})

	request := testRequest()
	request.MaxTokens = 1024

	if _, err := client.Generate(t.Context(), request); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	got, _ := body["max_tokens"].(float64)
	if int(got) != 1024 {
		t.Fatalf("max_tokens = %v, want the caller's explicit budget of 1024 left untouched", body["max_tokens"])
	}
}

func TestGenerateServesCachedResponsesWithoutCallingTheProvider(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeStubPayload(w)
	}))
	defer server.Close()

	store := &fakeStore{cached: `{"name":"cached"}`}
	client := newTestClient(t, newStubProvider(Response{Content: `{"name":"live"}`}), server.URL, store)

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

// A switch of provider must not serve the other provider's answer: the prompt
// hash is salted with the provider name.
func TestPromptHashSeparatesProviders(t *testing.T) {
	first := newStubProvider(Response{Content: "{}"})
	second := newStubProvider(Response{Content: "{}"})
	second.name = "other-stub"

	request := testRequest()
	firstHash := newTestClient(t, first, "http://localhost", &fakeStore{}).promptHash(request)
	secondHash := newTestClient(t, second, "http://localhost", &fakeStore{}).promptHash(request)

	if firstHash == secondHash {
		t.Fatal("prompt hashes match across providers, want a per-provider cache key")
	}
}

func TestGenerateRetriesTransientProviderFailures(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeStubPayload(w)
	}))
	defer server.Close()

	client := newTestClient(t, newStubProvider(Response{Content: `{"name":"third try"}`}), server.URL, &fakeStore{})

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
	// 529 is Anthropic's overload code and 502 a gateway failure in front of any
	// provider; the core retries both without knowing whose they are.
	for _, status := range []int{statusOverloadedCode, http.StatusBadGateway} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(status)
			}))
			defer server.Close()

			store := &fakeStore{}
			client := newTestClient(t, newStubProvider(Response{Content: "{}"}), server.URL, store)

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
		})
	}
}

func TestGenerateDoesNotRetryClientErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, newStubProvider(Response{Content: "{}"}), server.URL, &fakeStore{})

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a provider error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want no retry for a permanent client error", attempts.Load())
	}
}

// An answer the provider cannot decode is logged as a failure, not parsed: the
// core does not care why the wire format rejected it.
func TestGenerateFailsWhenTheProviderCannotDecodeTheAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeStubPayload(w)
	}))
	defer server.Close()

	provider := newStubProvider(Response{})
	provider.err = ErrNoStructuredOutput
	store := &fakeStore{}
	client := newTestClient(t, provider, server.URL, store)

	_, err := client.Generate(t.Context(), testRequest())

	if !errors.Is(err, ErrNoStructuredOutput) {
		t.Fatalf("Generate() error = %v, want the provider's decode failure", err)
	}
	if len(store.calls) != 1 || store.calls[0].Status != StatusError {
		t.Fatalf("logged calls = %#v, want the unusable response logged as an error", store.calls)
	}
}

func TestGenerateHonoursTheConfiguredTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		writeStubPayload(w)
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	config := testConfig(server.URL)
	config.Timeout = 20 * time.Millisecond
	config.MaxAttempts = 1
	client, err := New(config, &fakeStore{}, newStubProvider(Response{Content: "{}"}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a timeout error")
	}
}

func TestNewRejectsIncompleteConfiguration(t *testing.T) {
	cases := map[string]Config{
		"no model":   {Endpoint: "http://localhost", APIKey: "k"},
		"no api key": {Model: "m", Endpoint: "http://localhost"},
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			client, err := New(config, &fakeStore{}, newStubProvider(Response{}))

			var appError *apperr.Error
			if !errors.As(err, &appError) || appError.Code != apperr.CodeValidation {
				t.Fatalf("New() error = %v, want a validation error for %s", err, name)
			}
			if client != nil {
				t.Fatalf("New() = %#v, want no client for %s", client, name)
			}
		})
	}
}

// An endpoint is only needed for a proxy or a gateway; without one the provider
// supplies its own.
func TestNewFallsBackToTheProviderEndpoint(t *testing.T) {
	provider := newStubProvider(Response{})

	client, err := New(testConfig(""), &fakeStore{}, provider)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.config.Endpoint != provider.DefaultEndpoint() {
		t.Fatalf("endpoint = %q, want the provider default %q",
			client.config.Endpoint, provider.DefaultEndpoint())
	}
}

func TestNewKeepsAnExplicitEndpoint(t *testing.T) {
	client, err := New(testConfig("http://gateway.internal/v1"), &fakeStore{}, newStubProvider(Response{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.config.Endpoint != "http://gateway.internal/v1" {
		t.Fatalf("endpoint = %q, want the configured gateway", client.config.Endpoint)
	}
}

// The audit row must name the provider that answered, never one the config
// merely asked for.
func TestNewRecordsTheProviderThatAnswered(t *testing.T) {
	config := testConfig("http://localhost")
	config.Provider = "something-else"

	client, err := New(config, &fakeStore{}, newStubProvider(Response{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.config.Provider != "stub" {
		t.Fatalf("provider = %q, want the provider that answered", client.config.Provider)
	}
}

func newTestClient(t *testing.T, provider Provider, endpoint string, store CallStore) *Client {
	t.Helper()
	client, err := New(testConfig(endpoint), store, provider)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}
