package analytics

import "context"

// GetNutritionQualityAnalytics — quality-аналитика + coverage + нагрузка fixed meals. TODO.
type GetNutritionQualityAnalytics struct{ repo AnalyticsRepository }

// NewGetNutritionQualityAnalytics собирает use case.
func NewGetNutritionQualityAnalytics(repo AnalyticsRepository) *GetNutritionQualityAnalytics {
	return &GetNutritionQualityAnalytics{repo: repo}
}

// Handle — TODO.
func (uc *GetNutritionQualityAnalytics) Handle(_ context.Context, _ any) (any, error) {
	return nil, nil
}
