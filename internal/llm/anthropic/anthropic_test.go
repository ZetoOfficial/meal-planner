package anthropic

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

func TestEncodesToolUse(t *testing.T) {
	payload, err := New().EncodeRequest(testConfig(""), testRequest())
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	var body struct {
		Model       string  `json:"model"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		System      string  `json:"system"`
		Messages    []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"input_schema"`
		} `json:"tools"`
		ToolChoice struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}

	if body.Model != "test-model" || body.Temperature != 0.2 || body.MaxTokens != 1024 {
		t.Fatalf("body = %#v, want the configured model, temperature and token budget", body)
	}
	if body.System != "You generate recipes." || len(body.Messages) != 1 || body.Messages[0].Role != "user" {
		t.Fatalf("body = %#v, want the system prompt and one user message", body)
	}
	if len(body.Tools) != 1 || body.Tools[0].Name != "emit_recipe" || len(body.Tools[0].InputSchema) == 0 {
		t.Fatalf("tools = %#v, want the schema carried as the only tool", body.Tools)
	}
	if body.ToolChoice.Type != "tool" || body.ToolChoice.Name != "emit_recipe" {
		t.Fatalf("tool_choice = %#v, want the tool forced by name", body.ToolChoice)
	}
}

func TestEncodesPlainPromptsWithoutTools(t *testing.T) {
	request := testRequest()
	request.Schema = nil
	request.ToolName = ""

	payload, err := New().EncodeRequest(testConfig(""), request)
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	var body struct {
		Tools      []json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage   `json:"tool_choice"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
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
		t.Fatalf("content = %q, want the tool arguments", response.Content)
	}
	if response.TokensIn != 120 || response.TokensOut != 45 {
		t.Fatalf("usage = %d/%d, want 120/45", response.TokensIn, response.TokensOut)
	}
}

func TestRejectsAnswersWithoutTheRequestedTool(t *testing.T) {
	cases := map[string][]byte{
		"prose":      textPayload(t),
		"other tool": toolUsePayload(t, `{"name":"wrong tool"}`, 1, 1),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			toolName := "emit_recipe"
			if name == "other tool" {
				toolName = "emit_shopping_list"
			}
			_, err := New().DecodeResponse(payload, toolName)
			if err == nil {
				t.Fatal("DecodeResponse() = nil error, want an error for an unusable answer")
			}
			// Prose specifically must fail with the shared sentinel, not a
			// local error: the core matches on it with errors.Is to decide how
			// to log the call, so a package-local error here would still pass
			// every other check while breaking that classification silently.
			if name == "prose" && !errors.Is(err, llm.ErrNoStructuredOutput) {
				t.Fatalf("DecodeResponse() error = %v, want %v", err, llm.ErrNoStructuredOutput)
			}
		})
	}
}

func TestAuthorizesWithTheAPIKeyHeader(t *testing.T) {
	httpRequest := httptest.NewRequest(http.MethodPost, "http://localhost", http.NoBody)
	New().Authorize(httpRequest, testConfig(""))

	if got := httpRequest.Header.Get("x-api-key"); got != "test-key" {
		t.Fatalf("x-api-key = %q, want the configured key", got)
	}
	if got := httpRequest.Header.Get("anthropic-version"); got != apiVersion {
		t.Fatalf("anthropic-version = %q, want %q", got, apiVersion)
	}
}

func TestNewReportsItsNameAndEndpoint(t *testing.T) {
	// Asserted against literals, not the constants the methods return: a
	// rename of Name or defaultEndpoint must fail this test, the way it would
	// break the config-facing provider name or an already-deployed endpoint.
	if got := New().Name(); got != "anthropic" {
		t.Fatalf("Name() = %q, want %q", got, "anthropic")
	}
	if got := New().DefaultEndpoint(); got != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("DefaultEndpoint() = %q, want %q", got, "https://api.anthropic.com/v1/messages")
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
		"content": []map[string]any{
			{"type": "tool_use", "name": "emit_recipe", "input": json.RawMessage(arguments)},
		},
		"usage": map[string]int{"input_tokens": tokensIn, "output_tokens": tokensOut},
	})
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
}

func textPayload(t *testing.T) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": "sorry"}},
		"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
	})
	if err != nil {
		t.Fatalf("encode provider payload: %v", err)
	}
	return encoded
}

// noopStore is a CallStore that remembers nothing: the cache and the audit log
// are the core's tests to run, not this package's.
type noopStore struct{}

func (noopStore) Lookup(context.Context, llm.CacheKey) (response string, found bool, err error) {
	return "", false, nil
}
func (noopStore) SaveCache(context.Context, llm.CacheEntry) error { return nil }
func (noopStore) LogCall(context.Context, llm.CallRecord) error   { return nil }
