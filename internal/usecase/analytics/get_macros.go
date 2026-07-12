package analytics

import "context"

type GetMacroAnalytics struct{ repo AnalyticsRepository }

func NewGetMacroAnalytics(repo AnalyticsRepository) *GetMacroAnalytics {
	return &GetMacroAnalytics{repo: repo}
}

func (uc *GetMacroAnalytics) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
