package shopping

import (
	"testing"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
)

func TestBuildAggregatesOneRowPerProduct(t *testing.T) {
	list := Build(planWithFills(), products(), List{})

	if len(list.Items) != 2 {
		t.Fatalf("items = %d, want one row per product", len(list.Items))
	}
	chicken := itemFor(t, list, 1)
	if chicken.TotalQtyG != 500 {
		t.Fatalf("chicken quantity = %v g, want the summed fills", chicken.TotalQtyG)
	}
	if chicken.EstCost == nil || *chicken.EstCost != 4 {
		t.Fatalf("chicken cost = %v, want 500 g at 0.80 per 100 g", chicken.EstCost)
	}
	if chicken.Unit != "g" {
		t.Fatalf("unit = %q, want the product default unit", chicken.Unit)
	}
	rice := itemFor(t, list, 2)
	if rice.EstCost != nil {
		t.Fatalf("rice cost = %v, want unknown rather than free", rice.EstCost)
	}
}

func TestBuildKeepsChecksOfUnchangedRows(t *testing.T) {
	previous := List{ID: 3, Items: []Item{
		{ID: 10, ProductID: 1, TotalQtyG: 500, Checked: true},
		{ID: 11, ProductID: 2, TotalQtyG: 100, Checked: true},
	}}

	list := Build(planWithFills(), products(), previous)

	if !itemFor(t, list, 1).Checked {
		t.Fatal("unchanged row lost its check")
	}
	if itemFor(t, list, 2).Checked {
		t.Fatal("row with a changed quantity kept its check, want it reset")
	}
	if itemFor(t, list, 1).ID != 10 {
		t.Fatalf("item id = %d, want the stored row reused", itemFor(t, list, 1).ID)
	}
}

func TestBuildDropsProductsNoLongerInThePlan(t *testing.T) {
	previous := List{Items: []Item{{ID: 12, ProductID: 9, TotalQtyG: 50, Checked: true}}}

	list := Build(planWithFills(), products(), previous)

	for _, item := range list.Items {
		if item.ProductID == 9 {
			t.Fatalf("list still contains %#v, want removed products dropped", item)
		}
	}
}

func TestEstimatedTotalStaysUnknownWithoutPrices(t *testing.T) {
	list := List{Items: []Item{{ProductID: 2, TotalQtyG: 200}}}

	if total := list.EstimatedTotal(); total != nil {
		t.Fatalf("EstimatedTotal() = %v, want unknown rather than zero", total)
	}
}

func itemFor(t *testing.T, list List, productID catalogdom.ProductID) Item {
	t.Helper()
	for _, item := range list.Items {
		if item.ProductID == productID {
			return item
		}
	}
	t.Fatalf("list has no product %d", productID)
	return Item{}
}

func planWithFills() planningdom.Plan {
	return planningdom.Plan{
		ID: 7,
		Recipes: []planningdom.PlanRecipe{
			{ID: 11, Fills: []planningdom.SlotFill{
				{ID: 1, ProductID: 1, PlannedInputQtyG: 300},
				{ID: 2, ProductID: 2, PlannedInputQtyG: 150},
			}},
			{ID: 12, Fills: []planningdom.SlotFill{
				{ID: 3, ProductID: 1, PlannedInputQtyG: 200},
			}},
		},
	}
}

func products() map[catalogdom.ProductID]catalogdom.Product {
	price := 0.8
	return map[catalogdom.ProductID]catalogdom.Product{
		1: {ID: 1, Name: "Chicken breast", DefaultUnit: "g", PricePer100G: &price},
		2: {ID: 2, Name: "Rice", DefaultUnit: "g"},
	}
}
