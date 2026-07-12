package catalog

type ProductID int64

type Product struct {
	ID    ProductID
	Brand string
	Name  string
}
