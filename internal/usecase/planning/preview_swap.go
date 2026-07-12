package planning

import "context"

type PreviewSwap struct{}

func (uc *PreviewSwap) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
