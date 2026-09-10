package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

func TestEncodesFunctionCalling(t *testing.T) {
	payload, err := New().EncodeRequest(testConfig(""), testRequest())
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	var body struct {
		Model       string  `json:"model"`
		MaxTokens   int     `json:"max_completion_tokens"`
		Temperature float64 `json:"temperature"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		ToolChoice struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}

	if body.Model != "test-model" || body.Temperature != 0.2 || body.MaxTokens != 1024 {
		t.Fatalf("body = %#v, want the configured model, temperature and token budget", body)
	}
	if len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" {
		t.Fatalf("messages = %#v, want the system prompt as its own message before the user turn", body.Messages)
	}
	if len(body.Tools) != 1 || body.Tools[0].Type != "function" ||
		body.Tools[0].Function.Name != "emit_recipe" || len(body.Tools[0].Function.Parameters) == 0 {
		t.Fatalf("tools = %#v, want the schema carried as the only function", body.Tools)
	}
	if body.ToolChoice.Type != "function" || body.ToolChoice.Function.Name != "emit_recipe" {
		t.Fatalf("tool_choice = %#v, want the function forced by name", body.ToolChoice)
	}
}

func TestEncodesPlainPromptsWithoutTools(t *testing.T) {
	request := testRequest()
	request.Schema = nil
	request.ToolName = ""
	request.System = ""

	payload, err := New().EncodeRequest(testConfig(""), request)
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	var body struct {
		Messages   []json.RawMessage `json:"messages"`
		Tools      []json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage   `json:"tool_choice"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if len(body.Messages) != 1 {
		t.Fatalf("messages = %d, want only the user turn without a system prompt", len(body.Messages))
	}
	if body.Tools != nil || body.ToolChoice != nil {
		t.Fatalf("body = %#v, want no tools when the request carries no schema", body)
	}
}

func TestDecodesStructuredOutputAndUsage(t *testing.T) {
	response, err := New().DecodeResponse(toolUsePayload(t, `{"name":"Chicken bowl"}`, 120, 45), "emit_recipe")
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}
	if response.Content != `{"name":"Chicken bowl"}` {
		t.Fatalf("content = %q, want the function arguments", response.Content)
	}
	if response.TokensIn != 120 || response.TokensOut != 45 {
		t.Fatalf("usage = %d/%d, want 120/45", response.TokensIn, response.TokensOut)
	}
}

func TestRejectsAnswersWithoutTheRequestedFunction(t *testing.T) {
	t.Run("prose", func(t *testing.T) {
		// errors.Is, not just non-nil: this pins the provider to the core's
		// shared sentinel, so a local one introduced later fails the test
		// instead of passing it.
		if _, err := New().DecodeResponse(textPayload(t), "emit_recipe"); !errors.Is(err, llm.ErrNoStructuredOutput) {
			t.Fatalf("DecodeResponse() error = %v, want llm.ErrNoStructuredOutput for prose", err)
		}
	})
	t.Run("other function", func(t *testing.T) {
		// Same sentinel as the prose case: toolArguments has exactly one failure
		// path, so both subtests pin it and neither can drift onto a local error.
		payload := toolUsePayload(t, `{"name":"wrong tool"}`, 1, 1)
		if _, err := New().DecodeResponse(payload, "emit_shopping_list"); !errors.Is(err, llm.ErrNoStructuredOutput) {
			t.Fatalf("DecodeResponse() error = %v, want llm.ErrNoStructuredOutput when another function answered", err)
		}
	})
}

// The literals, not the constants: comparing a method to the constant it
// returns cannot fail, and these two strings are what config files and the
// composition root are written against.
func TestNewReportsItsNameAndEndpoint(t *testing.T) {
	if got := New().Name(); got != "openai" {
		t.Fatalf("Name() = %q, want %q", got, "openai")
	}
	if got := New().DefaultEndpoint(); got != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("DefaultEndpoint() = %q, want the documented Chat Completions url", got)
	}
}

func TestAuthorizesWithABearerToken(t *testing.T) {
	httpRequest := httptest.NewRequest(http.MethodPost, "http://localhost", http.NoBody)
	New().Authorize(httpRequest, testConfig(""))

	if got := httpRequest.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want a bearer token", got)
	}
}

// The provider has to survive the real transport, not just its own unit tests.
func TestRoundTripThroughTheClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(toolUsePayload(t, `{"name":"Chicken bowl"}`, 7, 3))
	}))
	defer server.Close()

	client, err := llm.New(testConfig(server.URL), noopStore{}, New())
	if err != nil {
		t.Fatalf("llm.New() error = %v", err)
	}

	response, err := client.Generate(t.Context(), testRequest())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Content != `{"name":"Chicken bowl"}` || response.TokensIn != 7 || response.TokensOut != 3 {
		t.Fatalf("response = %#v, want the decoded candidate and its usage", response)
	}
}

func testConfig(endpoint string) llm.Config {
	return llm.Config{
		Model:       "test-model",
		Endpoint:    endpoint,
		APIKey:      "test-key",
		Timeout:     2 * time.Second,
		MaxAttempts: 1,
		BaseBackoff: time.Millisecond,
	}
}

func testRequest() llm.Request {
	return llm.Request{
		System:      "You generate recipes.",
		Prompt:      "Generate one recipe using chicken.",
		ToolName:    "emit_recipe",
		Schema:      json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
		Temperature: 0.2,
		MaxTokens:   1024,
	}
}

func toolUsePayload(t *testing.T, arguments string, tokensIn, tokensOut int) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{
			"tool_calls": []map[string]any{{
				"type":     "function",
				"function": map[string]string{"name": "emit_recipe", "arguments": arguments},
			}},
		}}},
		"usage": map[string]int{"prompt_tokens": tokensIn, "completion_tokens": tokensOut},
	})
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
}

func textPayload(t *testing.T) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": "sorry"}}},
		"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
	})
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
}

// noopStore is a CallStore that remembers nothing: the cache and the audit log
// are the core's tests to run, not this package's.
type noopStore struct{}

// Named results because gocritic's unnamedResult fires on a three-value return
// here, and the CallStore interface names them too.
func (noopStore) Lookup(context.Context, llm.CacheKey) (response string, found bool, err error) {
	return "", false, nil
}
func (noopStore) SaveCache(context.Context, llm.CacheEntry) error { return nil }
func (noopStore) LogCall(context.Context, llm.CallRecord) error   { return nil }
