package planning

import "context"

// PreviewSwap — blast radius до подтверждения (diff БЖУ/качества, prep-граф). TODO.
type PreviewSwap struct{}

// Handle — TODO.
func (uc *PreviewSwap) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
