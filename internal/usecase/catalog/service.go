package catalog

// Service — use cases каталога (CreateProduct/UpdateProduct/...). TODO: методы.
type Service struct {
	products ProductRepository
}

// NewService собирает сервис каталога.
func NewService(products ProductRepository) *Service { return &Service{products: products} }
