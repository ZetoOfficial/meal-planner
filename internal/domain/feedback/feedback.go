// Package feedback — отзыв меняет вес/вероятность рецепта или продукта, а не банит.
// В MVP комментарий хранится как текст; LLM-парсинг — v2 (ARCHITECTURE §5.9).
package feedback

// TargetType — на что оставлен отзыв.
type TargetType string

// Event — событие фидбека. TODO: rating, comment, weight_delta (см. schema.sql).
type Event struct {
	TargetType TargetType
	TargetID   int64
}
