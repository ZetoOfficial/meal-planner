package analytics

import "context"

// GetMacroAnalytics — средние БЖУ (planned/actual) за период. TODO.
type GetMacroAnalytics struct{ repo AnalyticsRepository }

// NewGetMacroAnalytics собирает use case.
func NewGetMacroAnalytics(repo AnalyticsRepository) *GetMacroAnalytics { return &GetMacroAnalytics{repo: repo} }

// Handle — TODO.
func (uc *GetMacroAnalytics) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
