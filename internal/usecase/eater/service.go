package eater

type Service struct {
	eaters EaterRepository
}

func NewService(eaters EaterRepository) *Service { return &Service{eaters: eaters} }
