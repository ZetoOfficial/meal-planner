// Package analytics (usecase) — read-side: средние БЖУ, quality, топ рецептов.
package analytics

import "context"

// AnalyticsRepository — порт чтения агрегатов (read models, ARCHITECTURE §13).
type AnalyticsRepository interface {
	MacroAverages(ctx context.Context, eaterID int64, from, to string) (any, error)
	QualityAverages(ctx context.Context, eaterID int64, from, to string) (any, error)
	TopRecipes(ctx context.Context, limit int) (any, error)
}
