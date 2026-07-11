package catalog

type Service struct {
	products ProductRepository
}

func NewService(products ProductRepository) *Service { return &Service{products: products} }
