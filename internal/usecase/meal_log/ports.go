package meal_log

import "context"

type MealRepository interface {
	LogActual(ctx context.Context, mealID int64, payload any) error
}
