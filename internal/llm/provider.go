package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider is the wire format of one LLM API. The client owns the HTTP call,
// its timing, the cache and the retry budget, so a provider only has to encode
// a request and pull the structured answer back out of a payload.
type Provider interface {
	// Name identifies the provider in llm_call audit rows.
	Name() string
	// DefaultEndpoint is used when Config leaves the endpoint empty.
	DefaultEndpoint() string
	// EncodeRequest builds the JSON body of one completion call.
	EncodeRequest(config Config, request Request) ([]byte, error)
	// Authorize adds the provider's own authentication headers.
	Authorize(httpRequest *http.Request, config Config)
	// DecodeResponse extracts the structured content and the token usage. It
	// fails when the model answered outside the requested schema.
	DecodeResponse(payload []byte, toolName string) (Response, error)
}

// Provider names accepted by Config.Provider.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
)

const (
	defaultMaxTokens  = 4096
	maxErrorBodyBytes = 4 << 10
	// statusOverloadedCode is Anthropic's non-standard overload status; it is
	// retryable like a rate limit.
	statusOverloadedCode = 529
)

// errNoStructuredOutput is what both providers report when the model answered
// with prose instead of the schema: the candidate is unusable, so the call is
// logged as an error rather than parsed.
var errNoStructuredOutput = errors.New("provider answered without the requested structured output")

// providerFor resolves a provider name. A switch keeps the supported set
// visible in one place: no registry to consult and no init order to reason
// about. A custom wire format goes through NewWithProvider instead.
func providerFor(name string) (Provider, error) {
	switch name {
	case ProviderAnthropic:
		return anthropicProvider{}, nil
	case ProviderOpenAI:
		return openAIProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q, want %q or %q", name, ProviderAnthropic, ProviderOpenAI)
	}
}

// complete performs one provider call: the client times it and classifies the
// status, the provider decodes the payload.
func (c *Client) complete(ctx context.Context, request Request) (Response, error) {
	payload, err := c.provider.EncodeRequest(c.config, request)
	if err != nil {
		return Response{}, fmt.Errorf("encode provider request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("build provider request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	c.provider.Authorize(httpRequest, c.config)

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

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return Response{Latency: latency}, fmt.Errorf("read provider response: %w", err)
	}
	response, err := c.provider.DecodeResponse(body, request.ToolName)
	if err != nil {
		return Response{Latency: latency}, err
	}
	response.Latency = latency
	return response, nil
}

// retryableStatus reports the statuses worth another attempt: rate limits,
// overload and gateway trouble. Anything else is a request problem that a
// retry would only repeat.
func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		statusOverloadedCode:
		return true
	default:
		return false
	}
}

func providerStatusError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	if retryableStatus(response.StatusCode) {
		return &transientError{status: response.StatusCode}
	}
	return fmt.Errorf("provider returned status %d: %s", response.StatusCode, bytes.TrimSpace(body))
}

// requestMaxTokens keeps the output bound explicit for every provider.
func requestMaxTokens(request Request) int {
	if request.MaxTokens > 0 {
		return request.MaxTokens
	}
	return defaultMaxTokens
}
