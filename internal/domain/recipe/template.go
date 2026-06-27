// Package recipe — шаблоны рецептов и слоты. Заполнение слота продуктом происходит
// в плане; только permanent участвует в reuse-first без LLM (ARCHITECTURE §5.2).
package recipe

// ID — идентификатор шаблона.
type ID int64

// Status — жизненный цикл рецепта.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPermanent Status = "permanent"
	StatusRejected  Status = "rejected"
)

// Template — рецепт-шаблон со слотами. TODO: cuisine, method, equipment, weight.
type Template struct {
	ID     ID
	Name   string
	Status Status
	Slots  []Slot
}

// TemplateDraft — кандидат от LLM до валидации/повышения.
type TemplateDraft struct {
	Name  string
	Slots []Slot
}
