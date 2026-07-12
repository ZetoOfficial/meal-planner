package llm

import (
	"context"
	"errors"
	"time"
)

type Request struct {
	System      string
	Prompt      string
	Schema      []byte
	Temperature float64
}

type Response struct {
	Content   string
	Model     string
	TokensIn  int
	TokensOut int
	Latency   time.Duration
	Cached    bool
}

var ErrNotImplemented = errors.New("llm: provider not implemented")

type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

type StubClient struct {
	Model string
}

func (c StubClient) Generate(_ context.Context, _ Request) (Response, error) {
	return Response{Model: c.Model}, ErrNotImplemented
}

var _ Client = StubClient{}
