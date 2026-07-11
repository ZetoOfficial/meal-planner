package planning

import "context"

type ApplySwap struct{}

func (uc *ApplySwap) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
