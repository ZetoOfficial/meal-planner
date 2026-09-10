package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const testModel = "test-model"

// stubProvider is the core's stand-in for a wire format: the client's cache,
// retry budget and audit row must behave identically whatever the provider is,
// so the core's tests no longer name a real one.
type stubProvider struct {
	name     string
	endpoint string
	response Response
	err      error
}

func newStubProvider(response Response) *stubProvider {
	return &stubProvider{name: "stub", endpoint: "https://stub.invalid/v1", response: response}
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) DefaultEndpoint() string { return s.endpoint }

func (s *stubProvider) EncodeRequest(config Config, request Request) ([]byte, error) {
	// model and max_tokens are echoed because Client.complete is what fills
	// them in: TestGenerateReturnsStructuredContentAndLogsTheCall asserts that
	// the client hands its own Config to the provider (not a zero value, not
	// the request), and TestGenerateDefaultsMaxTokensWhenUnset asserts the
	// max_tokens value the client handed over. The stub is the only wire
	// format the core still has, so it is the only place left to prove either.
	return json.Marshal(map[string]any{
		"model":      config.Model,
		"prompt":     request.Prompt,
		"max_tokens": request.MaxTokens,
	})
}

func (s *stubProvider) Authorize(httpRequest *http.Request, config Config) {
	httpRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
}

func (s *stubProvider) DecodeResponse(_ []byte, _ string) (Response, error) {
	if s.err != nil {
		return Response{}, s.err
	}
	return s.response, nil
}

// writeStubPayload answers with the minimum a stub provider needs: the body is
// irrelevant because stubProvider.DecodeResponse ignores it.
func writeStubPayload(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func testConfig(endpoint string) Config {
	return Config{
		Model:       testModel,
		Endpoint:    endpoint,
		APIKey:      "test-key",
		Timeout:     2 * time.Second,
		MaxAttempts: 3,
		BaseBackoff: time.Millisecond,
	}
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
