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
			name:     ProviderAnthropic,
			provider: anthropicProvider{},
			toolUsePayload: func(t *testing.T, arguments string, tokensIn, tokensOut int) []byte {
				t.Helper()
				return encodeFixture(t, map[string]any{
					"content": []map[string]any{
						{"type": "tool_use", "name": "emit_recipe", "input": json.RawMessage(arguments)},
					},
					"usage": map[string]int{"input_tokens": tokensIn, "output_tokens": tokensOut},
				})
			},
			textPayload: func(t *testing.T, text string) []byte {
				t.Helper()
				return encodeFixture(t, map[string]any{
					"content": []map[string]any{{"type": "text", "text": text}},
					"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
				})
			},
		},
		{
			name:     ProviderOpenAI,
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

func encodeFixture(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
}

func writePayload(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

func testConfig(provider, endpoint string) Config {
	return Config{
		Provider:    provider,
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
