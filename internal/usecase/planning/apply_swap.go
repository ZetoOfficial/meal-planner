package planning

import "context"

// ApplySwap — применение свопа после подтверждения + пометка списка покупок stale. TODO.
type ApplySwap struct{}

// Handle — TODO.
func (uc *ApplySwap) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
