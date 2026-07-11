package analytics

import "context"

type AnalyticsRepository interface {
	MacroAverages(ctx context.Context, eaterID int64, from, to string) (any, error)
	QualityAverages(ctx context.Context, eaterID int64, from, to string) (any, error)
	TopRecipes(ctx context.Context, limit int) (any, error)
}
