package shopping

import (
	"math"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
)

const (
	gramsPer100 = 100.0
	// centScale and gramScale round money to cents and quantities to 0.1 g,
	// matching the stored numeric precision.
	centScale = 100.0
	gramScale = 10.0
)

type ListID int64

type ItemID int64

type List struct {
	ID     ListID
	PlanID planningdom.PlanID
	Items  []Item
}

type Item struct {
	ID          ItemID
	ProductID   catalogdom.ProductID
	ProductName string
	TotalQtyG   float64
	Unit        string
	EstCost     *float64
	Checked     bool
}

// EstimatedTotal sums the known item costs. It is nil when nothing is priced,
// because a zero total would read as "free" rather than "unknown".
func (l List) EstimatedTotal() *float64 {
	var total float64
	known := false
	for _, item := range l.Items {
		if item.EstCost == nil {
			continue
		}
		known = true
		total += *item.EstCost
	}
	if !known {
		return nil
	}
	return &total
}

// Build derives the list from a plan: one row per product with the summed
// planned input quantity and, when the product is priced, an estimated cost.
// Checked marks of unchanged rows survive a regeneration.
func Build(
	plan planningdom.Plan,
	products map[catalogdom.ProductID]catalogdom.Product,
	previous List,
) List {
	quantities := make(map[catalogdom.ProductID]float64)
	order := make([]catalogdom.ProductID, 0)
	for i := range plan.Recipes {
		for _, fill := range plan.Recipes[i].Fills {
			if _, seen := quantities[fill.ProductID]; !seen {
				order = append(order, fill.ProductID)
			}
			quantities[fill.ProductID] += fill.PlannedInputQtyG
		}
	}

	kept := previousByProduct(previous)
	list := List{ID: previous.ID, PlanID: plan.ID, Items: make([]Item, 0, len(order))}
	for _, productID := range order {
		product := products[productID]
		quantity := round1(quantities[productID])
		item := Item{
			ProductID:   productID,
			ProductName: product.Name,
			TotalQtyG:   quantity,
			Unit:        unitOf(product),
			EstCost:     estimatedCost(product, quantity),
		}
		if earlier, ok := kept[productID]; ok {
			item.ID = earlier.ID
			// A quantity change means a different purchase, so the tick resets.
			item.Checked = earlier.Checked && earlier.TotalQtyG == quantity
		}
		list.Items = append(list.Items, item)
	}
	return list
}

func previousByProduct(previous List) map[catalogdom.ProductID]Item {
	result := make(map[catalogdom.ProductID]Item, len(previous.Items))
	for _, item := range previous.Items {
		result[item.ProductID] = item
	}
	return result
}

func estimatedCost(product catalogdom.Product, quantityG float64) *float64 {
	if product.PricePer100G == nil {
		return nil
	}
	cost := math.Round(*product.PricePer100G*quantityG/gramsPer100*centScale) / centScale
	return &cost
}

func unitOf(product catalogdom.Product) string {
	if product.DefaultUnit == "" {
		return "g"
	}
	return product.DefaultUnit
}

func round1(value float64) float64 {
	return math.Round(value*gramScale) / gramScale
}
