// Package anthropic is the Messages API wire format. Structured output goes
// through tool use: the schema is the only shape the model may answer with, so
// Go unmarshals the arguments instead of parsing prose.
package anthropic

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

// Name identifies the provider in config and audit rows.
const Name = "anthropic"

const (
	apiVersion      = "2023-06-01"
	defaultEndpoint = "https://api.anthropic.com/v1/messages"
)

// New builds the provider. It holds no state: the client owns the HTTP
// transport, the cache and the retry budget.
func New() llm.Provider { return provider{} }

type provider struct{}

func (provider) Name() string { return Name }

func (provider) DefaultEndpoint() string { return defaultEndpoint }

func (provider) Authorize(httpRequest *http.Request, config llm.Config) {
	httpRequest.Header.Set("x-api-key", config.APIKey)
	httpRequest.Header.Set("anthropic-version", apiVersion)
}

func (provider) EncodeRequest(config llm.Config, request llm.Request) ([]byte, error) {
	body := requestBody{
		Model:       config.Model,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		System:      request.System,
		Messages:    []message{{Role: "user", Content: request.Prompt}},
	}
	if len(request.Schema) > 0 && request.ToolName != "" {
		body.Tools = []tool{{Name: request.ToolName, InputSchema: request.Schema}}
		body.ToolChoice = &toolChoice{Type: "tool", Name: request.ToolName}
	}
	return json.Marshal(body)
}

func (provider) DecodeResponse(payload []byte, toolName string) (llm.Response, error) {
	var decoded response
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return llm.Response{}, fmt.Errorf("decode provider response: %w", err)
	}
	content, err := decoded.toolArguments(toolName)
	if err != nil {
		return llm.Response{}, err
	}
	return llm.Response{
		Content:   content,
		TokensIn:  decoded.Usage.InputTokens,
		TokensOut: decoded.Usage.OutputTokens,
	}, nil
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestBody struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature"`
	System      string      `json:"system,omitempty"`
	Messages    []message   `json:"messages"`
	Tools       []tool      `json:"tools,omitempty"`
	ToolChoice  *toolChoice `json:"tool_choice,omitempty"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Text  string          `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type response struct {
	Content []contentBlock `json:"content"`
	Usage   usage          `json:"usage"`
}

func (r response) toolArguments(toolName string) (string, error) {
	for _, block := range r.Content {
		if block.Type == "tool_use" && (toolName == "" || block.Name == toolName) {
			return string(block.Input), nil
		}
	}
	return "", llm.ErrNoStructuredOutput
}
