package analytics

import "context"

type GetTopRecipes struct{ repo AnalyticsRepository }

func NewGetTopRecipes(repo AnalyticsRepository) *GetTopRecipes { return &GetTopRecipes{repo: repo} }

func (uc *GetTopRecipes) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
