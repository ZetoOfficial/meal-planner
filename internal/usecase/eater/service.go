package eater

// Service — use cases едока (CreateEater/CreateMacroGoalVersion/...). TODO.
type Service struct {
	eaters EaterRepository
}

// NewService собирает сервис едоков.
func NewService(eaters EaterRepository) *Service { return &Service{eaters: eaters} }
