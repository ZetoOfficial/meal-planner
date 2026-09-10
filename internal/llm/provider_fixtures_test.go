package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

const testModel = "test-model"

// providerFixture builds the payloads one provider would answer with, so the
// same client behaviour can be asserted against every wire format.
type providerFixture struct {
	name     string
	provider Provider
	// toolUsePayload is a well-formed structured answer from "emit_recipe".
	toolUsePayload func(t *testing.T, arguments string, tokensIn, tokensOut int) []byte
	// textPayload is prose, which the client must reject as unusable.
	textPayload func(t *testing.T, text string) []byte
}

func providerFixtures() []providerFixture {
	return []providerFixture{
		{
			name:     OpenAIName,
			provider: openAIProvider{},
			toolUsePayload: func(t *testing.T, arguments string, tokensIn, tokensOut int) []byte {
				t.Helper()
				return encodeFixture(t, map[string]any{
					"choices": []map[string]any{{"message": map[string]any{
						"tool_calls": []map[string]any{{
							"type":     "function",
							"function": map[string]string{"name": "emit_recipe", "arguments": arguments},
						}},
					}}},
					"usage": map[string]int{"prompt_tokens": tokensIn, "completion_tokens": tokensOut},
				})
			},
			textPayload: func(t *testing.T, text string) []byte {
				t.Helper()
				return encodeFixture(t, map[string]any{
					"choices": []map[string]any{{"message": map[string]any{"content": text}}},
					"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
				})
			},
		},
	}
}

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

func (s *stubProvider) EncodeRequest(_ Config, request Request) ([]byte, error) {
	// max_tokens is echoed because Client.complete is what fills that field in:
	// TestGenerateDefaultsMaxTokensWhenUnset asserts the value the client handed
	// over, and the stub is the only wire format the core still has.
	return json.Marshal(map[string]any{
		"model":      "stub",
		"prompt":     request.Prompt,
		"max_tokens": request.MaxTokens,
	})
}

func (s *stubProvider) Authorize(httpRequest *http.Request, config Config) {
	httpRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
}

func (s *stubProvider) DecodeResponse(payload []byte, _ string) (Response, error) {
	if s.err != nil {
		return Response{}, s.err
	}
	response := s.response
	if response.Content == "" {
		response.Content = string(payload)
	}
	return response, nil
}

// writeStubPayload answers with the minimum a stub provider needs: the body is
// irrelevant because stubProvider.DecodeResponse ignores it.
func writeStubPayload(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func encodeFixture(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
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
		// EncodeRequest now trusts the field as-is: Client.complete is the only
		// place that fills in the default, so a request built for a direct
		// EncodeRequest call must already carry one.
		MaxTokens: defaultMaxTokens,
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
