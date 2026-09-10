package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const openAIDefaultEndpoint = "https://api.openai.com/v1/chat/completions"

// openAIProvider speaks the Chat Completions API, which OpenAI-compatible
// gateways expose as well. Structured output goes through function calling on
// the same JSON schema Anthropic gets as a tool, so the recipe generator stays
// provider-agnostic and the schema is written once.
//
// Reasoning models (o-series, gpt-5) accept only temperature 1 and reject the
// novelty dial from DESIGN §11; that is a model choice, not an adapter concern.
type openAIProvider struct{}

func (openAIProvider) Name() string { return ProviderOpenAI }

func (openAIProvider) DefaultEndpoint() string { return openAIDefaultEndpoint }

func (openAIProvider) Authorize(httpRequest *http.Request, config Config) {
	httpRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
}

// EncodeRequest carries the system prompt as its own message, which is where
// Chat Completions expects it, and forces the function so the model cannot
// answer with prose.
func (openAIProvider) EncodeRequest(config Config, request Request) ([]byte, error) {
	messages := make([]openAIMessage, 0, 2)
	if request.System != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: request.System})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: request.Prompt})

	body := openAIRequestBody{
		Model:       config.Model,
		MaxTokens:   requestMaxTokens(request),
		Temperature: request.Temperature,
		Messages:    messages,
	}
	if len(request.Schema) > 0 && request.ToolName != "" {
		body.Tools = []openAITool{{
			Type:     "function",
			Function: openAIFunction{Name: request.ToolName, Parameters: request.Schema},
		}}
		body.ToolChoice = &openAIToolChoice{
			Type:     "function",
			Function: openAIFunctionRef{Name: request.ToolName},
		}
	}
	return json.Marshal(body)
}

func (openAIProvider) DecodeResponse(payload []byte, toolName string) (Response, error) {
	var decoded openAIResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Response{}, fmt.Errorf("decode provider response: %w", err)
	}
	content, err := decoded.toolArguments(toolName)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Content:   content,
		TokensIn:  decoded.Usage.PromptTokens,
		TokensOut: decoded.Usage.CompletionTokens,
	}, nil
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunctionRef struct {
	Name string `json:"name"`
}

type openAIToolChoice struct {
	Type     string            `json:"type"`
	Function openAIFunctionRef `json:"function"`
}

// openAIRequestBody uses max_completion_tokens: max_tokens is deprecated and
// rejected outright by the reasoning models.
type openAIRequestBody struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_completion_tokens"`
	Temperature float64           `json:"temperature"`
	Messages    []openAIMessage   `json:"messages"`
	Tools       []openAITool      `json:"tools,omitempty"`
	ToolChoice  *openAIToolChoice `json:"tool_choice,omitempty"`
}

type openAIFunctionCall struct {
	Name string `json:"name"`
	// Arguments arrives as a JSON string holding the schema instance.
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIResponseMessage struct {
	Content   string           `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls"`
}

type openAIChoice struct {
	Message openAIResponseMessage `json:"message"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

func (r openAIResponse) toolArguments(toolName string) (string, error) {
	for _, choice := range r.Choices {
		for _, call := range choice.Message.ToolCalls {
			if toolName == "" || call.Function.Name == toolName {
				return call.Function.Arguments, nil
			}
		}
	}
	return "", errNoStructuredOutput
}
