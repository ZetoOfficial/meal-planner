// Package shopping stores the shopping list derived from a plan.
package shopping

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

// Get returns the stored list, or a not found error when the plan has never
// had one generated.
func (r *Repository) Get(ctx context.Context, planID planningdom.PlanID) (shoppingdom.List, error) {
	query := r.store.Queries(ctx)
	row, err := query.GetShoppingListByPlan(ctx, int64(planID))
	if errors.Is(err, pgx.ErrNoRows) {
		return shoppingdom.List{}, apperr.NotFound("shopping list")
	}
	if err != nil {
		return shoppingdom.List{}, dberror.Map(err, "shopping list")
	}
	return r.load(ctx, query, row)
}

// Save replaces the list of a plan: rows for products that left the plan are
// deleted, the rest are upserted with their check state intact.
func (r *Repository) Save(ctx context.Context, list shoppingdom.List) (shoppingdom.List, error) {
	query := r.store.Queries(ctx)
	row, err := query.UpsertShoppingList(ctx, int64(list.PlanID))
	if err != nil {
		return shoppingdom.List{}, dberror.Map(err, "shopping list")
	}

	productIDs := make([]int64, 0, len(list.Items))
	for _, item := range list.Items {
		productIDs = append(productIDs, int64(item.ProductID))
	}
	if err := query.DeleteShoppingItemsOutsideProducts(ctx, sqlc.DeleteShoppingItemsOutsideProductsParams{
		ListID:     row.ID,
		ProductIds: productIDs,
	}); err != nil {
		return shoppingdom.List{}, dberror.Map(err, "shopping list item")
	}
	for _, item := range list.Items {
		if _, err := query.UpsertShoppingItem(ctx, sqlc.UpsertShoppingItemParams{
			ListID:    row.ID,
			ProductID: int64(item.ProductID),
			TotalQtyG: pgvalue.Numeric(item.TotalQtyG),
			Unit:      item.Unit,
			EstCost:   pgvalue.OptionalNumeric(item.EstCost),
			Checked:   item.Checked,
		}); err != nil {
			return shoppingdom.List{}, dberror.Map(err, "shopping list item")
		}
	}
	return r.load(ctx, query, row)
}

// SetChecked toggles one row without regenerating the list.
func (r *Repository) SetChecked(ctx context.Context, itemID shoppingdom.ItemID, checked bool) error {
	_, err := r.store.Queries(ctx).SetShoppingItemChecked(ctx, sqlc.SetShoppingItemCheckedParams{
		ID:      int64(itemID),
		Checked: checked,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("shopping list item")
	}
	if err != nil {
		return dberror.Map(err, "shopping list item")
	}
	return nil
}

// GetItem reads back one row so a check toggle can answer with the row itself.
func (r *Repository) GetItem(ctx context.Context, itemID shoppingdom.ItemID) (shoppingdom.Item, error) {
	row, err := r.store.Queries(ctx).GetShoppingItem(ctx, int64(itemID))
	if errors.Is(err, pgx.ErrNoRows) {
		return shoppingdom.Item{}, apperr.NotFound("shopping list item")
	}
	if err != nil {
		return shoppingdom.Item{}, dberror.Map(err, "shopping list item")
	}
	return itemFromRow(sqlc.ListShoppingItemsRow(row))
}

func itemFromRow(row sqlc.ListShoppingItemsRow) (shoppingdom.Item, error) {
	quantity, err := pgvalue.Float64(row.TotalQtyG)
	if err != nil {
		return shoppingdom.Item{}, err
	}
	cost, err := pgvalue.OptionalFloat64(row.EstCost)
	if err != nil {
		return shoppingdom.Item{}, err
	}
	return shoppingdom.Item{
		ID:          shoppingdom.ItemID(row.ID),
		ProductID:   catalogdom.ProductID(row.ProductID),
		ProductName: row.ProductName,
		TotalQtyG:   quantity,
		Unit:        row.Unit,
		EstCost:     cost,
		Checked:     row.Checked,
	}, nil
}

func (r *Repository) load(
	ctx context.Context,
	query *sqlc.Queries,
	row sqlc.ShoppingList,
) (shoppingdom.List, error) {
	rows, err := query.ListShoppingItems(ctx, row.ID)
	if err != nil {
		return shoppingdom.List{}, dberror.Map(err, "shopping list item")
	}
	list := shoppingdom.List{
		ID:     shoppingdom.ListID(row.ID),
		PlanID: planningdom.PlanID(row.PlanID),
		Items:  make([]shoppingdom.Item, 0, len(rows)),
	}
	for i := range rows {
		item, err := itemFromRow(rows[i])
		if err != nil {
			return shoppingdom.List{}, err
		}
		list.Items = append(list.Items, item)
	}
	return list, nil
}
