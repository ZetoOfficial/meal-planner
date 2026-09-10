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

// The cache, the retry budget and the audit log belong to the client, so every
// case below runs against each provider: a new wire format must not change any
// of them.
func TestGenerateReturnsStructuredContentAndLogsTheCall(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				writePayload(w, fixture.toolUsePayload(t, `{"name":"Chicken bowl"}`, 120, 45))
			}))
			defer server.Close()

			store := &fakeStore{}
			client := newTestClient(t, fixture.name, server.URL, store)

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
			if body["model"] != testModel || body["temperature"] != 0.2 {
				t.Fatalf("request body = %#v, want the configured model and temperature", body)
			}
			if len(store.calls) != 1 {
				t.Fatalf("logged calls = %d, want one", len(store.calls))
			}
			call := store.calls[0]
			if call.Status != StatusOK || call.TokensIn != 120 || call.TokensOut != 45 {
				t.Fatalf("logged call = %#v, want an ok call with token counts", call)
			}
			if call.Provider != fixture.name {
				t.Fatalf("logged provider = %q, want %q", call.Provider, fixture.name)
			}
			if call.PromptHash == "" || call.Prompt == "" || call.Response == "" {
				t.Fatalf("logged call = %#v, want the prompt and response recorded", call)
			}
			if len(store.saved) != 1 || store.saved[0].PromptHash != call.PromptHash {
				t.Fatalf("cached entries = %#v, want the response cached under its prompt hash", store.saved)
			}
		})
	}
}

func TestGenerateServesCachedResponsesWithoutCallingTheProvider(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				writePayload(w, fixture.toolUsePayload(t, `{"name":"live"}`, 1, 1))
			}))
			defer server.Close()

			store := &fakeStore{cached: `{"name":"cached"}`}
			client := newTestClient(t, fixture.name, server.URL, store)

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
		})
	}
}

// A switch of provider must not serve the other provider's answer: the prompt
// hash is salted with the provider name.
func TestPromptHashSeparatesProviders(t *testing.T) {
	request := testRequest()
	anthropic := newTestClient(t, ProviderAnthropic, "http://localhost", &fakeStore{})
	openAI := newTestClient(t, ProviderOpenAI, "http://localhost", &fakeStore{})

	if anthropic.promptHash(request) == openAI.promptHash(request) {
		t.Fatal("prompt hashes match across providers, want a per-provider cache key")
	}
}

func TestGenerateRetriesTransientProviderFailures(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if attempts.Add(1) < 3 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				writePayload(w, fixture.toolUsePayload(t, `{"name":"third try"}`, 10, 5))
			}))
			defer server.Close()

			client := newTestClient(t, fixture.name, server.URL, &fakeStore{})

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
		})
	}
}

func TestGenerateReportsUnavailableAfterExhaustingRetries(t *testing.T) {
	// 529 is Anthropic's overload code and 502 a gateway failure in front of
	// any provider; both are worth another attempt.
	for _, status := range []int{statusOverloadedCode, http.StatusBadGateway} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(status)
			}))
			defer server.Close()

			store := &fakeStore{}
			client := newTestClient(t, ProviderOpenAI, server.URL, store)

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

	client := newTestClient(t, ProviderAnthropic, server.URL, &fakeStore{})

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a provider error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want no retry for a permanent client error", attempts.Load())
	}
}

func TestGenerateFailsWhenTheProviderAnswersWithProse(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writePayload(w, fixture.textPayload(t, "sorry"))
			}))
			defer server.Close()

			store := &fakeStore{}
			client := newTestClient(t, fixture.name, server.URL, store)

			if _, err := client.Generate(t.Context(), testRequest()); err == nil {
				t.Fatal("Generate() = nil, want an error for a response without structured output")
			}
			if len(store.calls) != 1 || store.calls[0].Status != StatusError {
				t.Fatalf("logged calls = %#v, want the unusable response logged as an error", store.calls)
			}
		})
	}
}

func TestGenerateHonoursTheConfiguredTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		writePayload(w, []byte(`{}`))
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	config := testConfig(ProviderAnthropic, server.URL)
	config.Timeout = 20 * time.Millisecond
	config.MaxAttempts = 1
	client, err := New(config, &fakeStore{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := client.Generate(t.Context(), testRequest()); err == nil {
		t.Fatal("Generate() = nil, want a timeout error")
	}
}

func TestNewRejectsIncompleteConfiguration(t *testing.T) {
	cases := map[string]Config{
		"no model":           {Endpoint: "http://localhost", APIKey: "k"},
		"no api key":         {Model: "m", Endpoint: "http://localhost"},
		"unknown provider":   {Provider: "llama-on-a-toaster", Model: "m", APIKey: "k"},
		"provider case typo": {Provider: "OpenAI", Model: "m", APIKey: "k"},
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			client, err := New(config, &fakeStore{})
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
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			client, err := New(testConfig(fixture.name, ""), &fakeStore{})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if client.config.Endpoint != fixture.provider.DefaultEndpoint() {
				t.Fatalf("endpoint = %q, want the provider default %q",
					client.config.Endpoint, fixture.provider.DefaultEndpoint())
			}
		})
	}
}

func TestNewKeepsAnExplicitEndpoint(t *testing.T) {
	client, err := New(testConfig(ProviderOpenAI, "http://gateway.internal/v1/chat/completions"), &fakeStore{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.config.Endpoint != "http://gateway.internal/v1/chat/completions" {
		t.Fatalf("endpoint = %q, want the configured gateway", client.config.Endpoint)
	}
}

func TestNewDefaultsToAnthropic(t *testing.T) {
	config := testConfig("", "")
	client, err := New(config, &fakeStore{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.config.Provider != ProviderAnthropic {
		t.Fatalf("provider = %q, want %q", client.config.Provider, ProviderAnthropic)
	}
}

// NewWithProvider records the provider that actually answered, so an audit row
// cannot claim one that was never called.
func TestNewWithProviderOverridesTheConfiguredName(t *testing.T) {
	client, err := NewWithProvider(testConfig("anthropic", "http://localhost"), &fakeStore{}, openAIProvider{})
	if err != nil {
		t.Fatalf("NewWithProvider() error = %v", err)
	}
	if client.config.Provider != ProviderOpenAI {
		t.Fatalf("provider = %q, want %q", client.config.Provider, ProviderOpenAI)
	}
}

func newTestClient(t *testing.T, provider, endpoint string, store CallStore) *Client {
	t.Helper()
	client, err := New(testConfig(provider, endpoint), store)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}
