package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	anthropicVersion         = "2023-06-01"
	anthropicDefaultEndpoint = "https://api.anthropic.com/v1/messages"
)

// anthropicProvider speaks the Messages API. Structured output goes through
// tool use: the schema is the only shape the model may answer with, so Go
// unmarshals the arguments instead of parsing prose.
type anthropicProvider struct{}

func (anthropicProvider) Name() string { return ProviderAnthropic }

func (anthropicProvider) DefaultEndpoint() string { return anthropicDefaultEndpoint }

func (anthropicProvider) Authorize(httpRequest *http.Request, config Config) {
	httpRequest.Header.Set("x-api-key", config.APIKey)
	httpRequest.Header.Set("anthropic-version", anthropicVersion)
}

func (anthropicProvider) EncodeRequest(config Config, request Request) ([]byte, error) {
	body := anthropicRequestBody{
		Model:       config.Model,
		MaxTokens:   requestMaxTokens(request),
		Temperature: request.Temperature,
		System:      request.System,
		Messages:    []anthropicMessage{{Role: "user", Content: request.Prompt}},
	}
	if len(request.Schema) > 0 && request.ToolName != "" {
		body.Tools = []anthropicTool{{Name: request.ToolName, InputSchema: request.Schema}}
		body.ToolChoice = &anthropicToolChoice{Type: "tool", Name: request.ToolName}
	}
	return json.Marshal(body)
}

func (anthropicProvider) DecodeResponse(payload []byte, toolName string) (Response, error) {
	var decoded anthropicResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Response{}, fmt.Errorf("decode provider response: %w", err)
	}
	content, err := decoded.toolArguments(toolName)
	if err != nil {
		return Response{}, err
	}
	return Response{
		Content:   content,
		TokensIn:  decoded.Usage.InputTokens,
		TokensOut: decoded.Usage.OutputTokens,
	}, nil
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequestBody struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature float64              `json:"temperature"`
	System      string               `json:"system,omitempty"`
	Messages    []anthropicMessage   `json:"messages"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Text  string          `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Usage   anthropicUsage          `json:"usage"`
}

func (r anthropicResponse) toolArguments(toolName string) (string, error) {
	for _, block := range r.Content {
		if block.Type == "tool_use" && (toolName == "" || block.Name == toolName) {
			return string(block.Input), nil
		}
	}
	return "", errNoStructuredOutput
}
