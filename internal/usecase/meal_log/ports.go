// Package meal_log (usecase) — логирование факта еды (planned vs actual).
package meal_log

import "context"

// MealRepository — порт доступа к приёмам пищи.
type MealRepository interface {
	LogActual(ctx context.Context, mealID int64, payload any) error
}
