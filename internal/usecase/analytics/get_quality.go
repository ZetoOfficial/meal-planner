package analytics

import "context"

type GetNutritionQualityAnalytics struct{ repo AnalyticsRepository }

func NewGetNutritionQualityAnalytics(repo AnalyticsRepository) *GetNutritionQualityAnalytics {
	return &GetNutritionQualityAnalytics{repo: repo}
}

func (uc *GetNutritionQualityAnalytics) Handle(_ context.Context, _ any) (any, error) {
	return nil, nil
}
