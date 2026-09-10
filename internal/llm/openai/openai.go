// Package openai is the Chat Completions wire format, which OpenAI-compatible
// gateways expose as well. Structured output goes through function calling on
// the same JSON schema Anthropic gets as a tool, so the recipe generator stays
// provider-agnostic and the schema is written once.
//
// Reasoning models (o-series, gpt-5) accept only temperature 1 and reject the
// novelty dial from DESIGN §11; that is a model choice, not an adapter concern.
package openai

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ZetoOfficial/meal-planner/internal/llm"
)

// Name identifies the provider in config and audit rows.
const Name = "openai"

const defaultEndpoint = "https://api.openai.com/v1/chat/completions"

// New builds the provider. It holds no state: the client owns the HTTP
// transport, the cache and the retry budget.
func New() llm.Provider { return provider{} }

type provider struct{}

func (provider) Name() string { return Name }

func (provider) DefaultEndpoint() string { return defaultEndpoint }

func (provider) Authorize(httpRequest *http.Request, config llm.Config) {
	httpRequest.Header.Set("Authorization", "Bearer "+config.APIKey)
}

// EncodeRequest carries the system prompt as its own message, which is where
// Chat Completions expects it, and forces the function so the model cannot
// answer with prose.
func (provider) EncodeRequest(config llm.Config, request llm.Request) ([]byte, error) {
	messages := make([]message, 0, 2)
	if request.System != "" {
		messages = append(messages, message{Role: "system", Content: request.System})
	}
	messages = append(messages, message{Role: "user", Content: request.Prompt})

	body := requestBody{
		Model:       config.Model,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		Messages:    messages,
	}
	if len(request.Schema) > 0 && request.ToolName != "" {
		body.Tools = []tool{{
			Type:     "function",
			Function: function{Name: request.ToolName, Parameters: request.Schema},
		}}
		body.ToolChoice = &toolChoice{
			Type:     "function",
			Function: functionRef{Name: request.ToolName},
		}
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
		TokensIn:  decoded.Usage.PromptTokens,
		TokensOut: decoded.Usage.CompletionTokens,
	}, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type function struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type tool struct {
	Type     string   `json:"type"`
	Function function `json:"function"`
}

type functionRef struct {
	Name string `json:"name"`
}

type toolChoice struct {
	Type     string      `json:"type"`
	Function functionRef `json:"function"`
}

// requestBody uses max_completion_tokens: max_tokens is deprecated and
// rejected outright by the reasoning models.
type requestBody struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_completion_tokens"`
	Temperature float64     `json:"temperature"`
	Messages    []message   `json:"messages"`
	Tools       []tool      `json:"tools,omitempty"`
	ToolChoice  *toolChoice `json:"tool_choice,omitempty"`
}

type functionCall struct {
	Name string `json:"name"`
	// Arguments arrives as a JSON string holding the schema instance.
	Arguments string `json:"arguments"`
}

type toolCall struct {
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type responseMessage struct {
	Content   string     `json:"content"`
	ToolCalls []toolCall `json:"tool_calls"`
}

type choice struct {
	Message responseMessage `json:"message"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type response struct {
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

func (r response) toolArguments(toolName string) (string, error) {
	for _, entry := range r.Choices {
		for _, call := range entry.Message.ToolCalls {
			if toolName == "" || call.Function.Name == toolName {
				return call.Function.Arguments, nil
			}
		}
	}
	return "", llm.ErrNoStructuredOutput
}
