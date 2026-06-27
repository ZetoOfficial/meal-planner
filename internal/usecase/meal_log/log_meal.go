package meal_log

import "context"

// LogMeal — use case логирования: actual-макросы руками, quality → unknown (FR-6). TODO.
type LogMeal struct {
	meals MealRepository
}

// NewLogMeal собирает use case.
func NewLogMeal(meals MealRepository) *LogMeal { return &LogMeal{meals: meals} }

// Handle — TODO.
func (uc *LogMeal) Handle(_ context.Context, _ any) error { return nil }
