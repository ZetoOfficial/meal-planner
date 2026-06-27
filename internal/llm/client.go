// Package llm — обёртка над LLM-провайдером. Mental model: stateless HTTP,
// строка → строка (SPEC §11). Смена провайдера/fallback = одна имплементация.
package llm

import (
	"context"
	"errors"
	"time"
)

// Request — запрос на генерацию. Schema описывает ожидаемый structured output
// (tool use / function calling), Temperature — dial новизны (0–0.2 .. ~0.8).
type Request struct {
	System      string
	Prompt      string
	Schema      []byte // JSON Schema "инструмента" для structured output
	Temperature float64
}

// Response — ответ модели плюс метаданные для логирования (SPEC §11).
type Response struct {
	Content   string
	Model     string
	TokensIn  int
	TokensOut int
	Latency   time.Duration
	Cached    bool
}

// ErrNotImplemented возвращают заглушки до реализации провайдера.
var ErrNotImplemented = errors.New("llm: provider not implemented")

// Client — единый интерфейс генерации. Реализации: Anthropic, OpenAI (v2).
// Кэш по хешу промпта, retry на 429/529 и таймаут навешиваются обёртками.
type Client interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

// StubClient — заглушка для скелета: компилируется, ничего не вызывает.
// Заменить на реальную имплементацию (net/http к /v1/messages).
type StubClient struct {
	Model string
}

// Generate всегда возвращает ErrNotImplemented.
func (c StubClient) Generate(_ context.Context, _ Request) (Response, error) {
	return Response{Model: c.Model}, ErrNotImplemented
}

// Ensure StubClient реализует Client.
var _ Client = StubClient{}
