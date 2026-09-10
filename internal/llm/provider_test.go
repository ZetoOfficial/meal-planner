package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderForRejectsAnUnknownName(t *testing.T) {
	if _, err := providerFor("llama-on-a-toaster"); err == nil {
		t.Fatal("providerFor() = nil error, want an error for an unknown provider")
	}
}

func TestProviderForResolvesTheBuiltInProviders(t *testing.T) {
	for _, name := range []string{ProviderAnthropic, ProviderOpenAI} {
		t.Run(name, func(t *testing.T) {
			provider, err := providerFor(name)
			if err != nil {
				t.Fatalf("providerFor(%q) error = %v", name, err)
			}
			if provider.Name() != name {
				t.Fatalf("Name() = %q, want %q", provider.Name(), name)
			}
			if provider.DefaultEndpoint() == "" {
				t.Fatal("DefaultEndpoint() = empty, want the provider's own url")
			}
		})
	}
}

func TestAnthropicEncodesToolUse(t *testing.T) {
	payload, err := anthropicProvider{}.EncodeRequest(testConfig(ProviderAnthropic, ""), testRequest())
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
	decode(t, payload, &body)

	if body.Model != testModel || body.Temperature != 0.2 || body.MaxTokens != defaultMaxTokens {
		t.Fatalf("body = %#v, want the configured model, temperature and max tokens carried through", body)
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

func TestOpenAIEncodesFunctionCalling(t *testing.T) {
	payload, err := openAIProvider{}.EncodeRequest(testConfig(ProviderOpenAI, ""), testRequest())
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
	decode(t, payload, &body)

	if body.Model != testModel || body.Temperature != 0.2 || body.MaxTokens != defaultMaxTokens {
		t.Fatalf("body = %#v, want the configured model, temperature and max tokens carried through", body)
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

func TestOpenAIEncodesPlainPromptsWithoutTools(t *testing.T) {
	request := testRequest()
	request.Schema = nil
	request.ToolName = ""
	request.System = ""

	payload, err := openAIProvider{}.EncodeRequest(testConfig(ProviderOpenAI, ""), request)
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	var body struct {
		Messages   []json.RawMessage `json:"messages"`
		Tools      []json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage   `json:"tool_choice"`
	}
	decode(t, payload, &body)

	if len(body.Messages) != 1 {
		t.Fatalf("messages = %d, want only the user turn without a system prompt", len(body.Messages))
	}
	if body.Tools != nil || body.ToolChoice != nil {
		t.Fatalf("body = %#v, want no tools when the request carries no schema", body)
	}
}

func TestProvidersDecodeStructuredOutputAndUsage(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			response, err := fixture.provider.DecodeResponse(
				fixture.toolUsePayload(t, `{"name":"Chicken bowl"}`, 120, 45), "emit_recipe")
			if err != nil {
				t.Fatalf("DecodeResponse() error = %v", err)
			}
			if response.Content != `{"name":"Chicken bowl"}` {
				t.Fatalf("content = %q, want the tool arguments", response.Content)
			}
			if response.TokensIn != 120 || response.TokensOut != 45 {
				t.Fatalf("usage = %d/%d, want 120/45", response.TokensIn, response.TokensOut)
			}
		})
	}
}

func TestProvidersRejectAnswersWithoutStructuredOutput(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			if _, err := fixture.provider.DecodeResponse(fixture.textPayload(t, "sorry"), "emit_recipe"); err == nil {
				t.Fatal("DecodeResponse() = nil error, want an error for prose instead of structured output")
			}
		})
	}
}

func TestProvidersRejectStructuredOutputFromAnotherTool(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			payload := fixture.toolUsePayload(t, `{"name":"wrong tool"}`, 1, 1)
			if _, err := fixture.provider.DecodeResponse(payload, "emit_shopping_list"); err == nil {
				t.Fatal("DecodeResponse() = nil error, want an error when another tool answered")
			}
		})
	}
}

func TestProvidersAuthorizeWithTheirOwnHeaders(t *testing.T) {
	cases := map[string]struct {
		provider Provider
		header   string
		want     string
	}{
		ProviderAnthropic: {anthropicProvider{}, "x-api-key", "test-key"},
		ProviderOpenAI:    {openAIProvider{}, "Authorization", "Bearer test-key"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			httpRequest := httptest.NewRequest(http.MethodPost, "http://localhost", http.NoBody)
			testCase.provider.Authorize(httpRequest, testConfig(name, ""))
			if got := httpRequest.Header.Get(testCase.header); got != testCase.want {
				t.Fatalf("%s = %q, want %q", testCase.header, got, testCase.want)
			}
		})
	}
}

func decode(t *testing.T, payload []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
}
