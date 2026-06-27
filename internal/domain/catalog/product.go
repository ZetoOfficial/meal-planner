// Package catalog — личная база продуктов, роли, аллергены, food classification,
// nutrition facts. Не знает про планировщик (ARCHITECTURE §5.1).
package catalog

// ProductID — идентификатор продукта.
type ProductID int64

// Product — бренд с реальными БЖУ на 100 г и quality-полями. TODO: nutrition facts,
// classification, data-quality метаданные (см. docs/schema.sql).
type Product struct {
	ID    ProductID
	Brand string
	Name  string
}
