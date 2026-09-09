package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicVersion     = "2023-06-01"
	defaultMaxTokens     = 4096
	maxErrorBodyBytes    = 4 << 10
	statusOverloadedCode = 529
)

// complete performs one provider call. Structured output goes through tool use:
// the schema is the only shape the model may answer with, so Go unmarshals the
// arguments instead of parsing prose.
func (c *Client) complete(ctx context.Context, request Request) (Response, error) {
	payload, err := json.Marshal(anthropicRequest(c.config.Model, request))
	if err != nil {
		return Response{}, fmt.Errorf("encode provider request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("build provider request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", c.config.APIKey)
	httpRequest.Header.Set("anthropic-version", anthropicVersion)

	started := time.Now()
	httpResponse, err := c.http.Do(httpRequest)
	if err != nil {
		return Response{Latency: time.Since(started)}, fmt.Errorf("call provider: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close() }()

	latency := time.Since(started)
	if httpResponse.StatusCode != http.StatusOK {
		return Response{Latency: latency}, providerStatusError(httpResponse)
	}

	var decoded anthropicResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&decoded); err != nil {
		return Response{Latency: latency}, fmt.Errorf("decode provider response: %w", err)
	}
	content, err := decoded.toolArguments(request.ToolName)
	if err != nil {
		return Response{Latency: latency}, err
	}

	return Response{
		Content:   content,
		TokensIn:  decoded.Usage.InputTokens,
		TokensOut: decoded.Usage.OutputTokens,
		Latency:   latency,
	}, nil
}

// providerStatusError separates rate limiting and overload, which are worth a
// retry, from permanent request problems, which are not.
func providerStatusError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	status := response.StatusCode
	if status == http.StatusTooManyRequests || status == statusOverloadedCode ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout {
		return &transientError{status: status}
	}
	return fmt.Errorf("provider returned status %d: %s", status, bytes.TrimSpace(body))
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

func anthropicRequest(model string, request Request) anthropicRequestBody {
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	body := anthropicRequestBody{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: request.Temperature,
		System:      request.System,
		Messages:    []anthropicMessage{{Role: "user", Content: request.Prompt}},
	}
	if len(request.Schema) > 0 && request.ToolName != "" {
		body.Tools = []anthropicTool{{Name: request.ToolName, InputSchema: request.Schema}}
		body.ToolChoice = &anthropicToolChoice{Type: "tool", Name: request.ToolName}
	}
	return body
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
	return "", errors.New("provider answered without the requested structured output")
}
