package meal_log

import "context"

type LogMeal struct {
	meals MealRepository
}

func NewLogMeal(meals MealRepository) *LogMeal { return &LogMeal{meals: meals} }

func (uc *LogMeal) Handle(_ context.Context, _ any) error { return nil }
