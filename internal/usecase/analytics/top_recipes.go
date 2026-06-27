package analytics

import "context"

// GetTopRecipes — топ рецептов по числу готовок. TODO.
type GetTopRecipes struct{ repo AnalyticsRepository }

// NewGetTopRecipes собирает use case.
func NewGetTopRecipes(repo AnalyticsRepository) *GetTopRecipes { return &GetTopRecipes{repo: repo} }

// Handle — TODO.
func (uc *GetTopRecipes) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
