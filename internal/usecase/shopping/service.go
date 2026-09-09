// Package shopping turns a stored plan into a purchasable list.
package shopping

import (
	"context"
	"errors"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

type Service struct {
	lists        ListRepository
	plans        PlanRepository
	products     ProductRepository
	transactions TxRunner
}

func NewService(
	lists ListRepository,
	plans PlanRepository,
	products ProductRepository,
	transactions TxRunner,
) *Service {
	return &Service{lists: lists, plans: plans, products: products, transactions: transactions}
}

// GetList returns the stored list, generating it on first read so a plan always
// has a list to shop from.
func (s *Service) GetList(ctx context.Context, planID planningdom.PlanID) (shoppingdom.List, error) {
	list, err := s.lists.Get(ctx, planID)
	if err == nil {
		return list, nil
	}
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != apperr.CodeNotFound {
		return shoppingdom.List{}, err
	}
	return s.Regenerate(ctx, planID)
}

// Regenerate rebuilds the list from the plan. Rows whose product and quantity
// did not change keep their check mark, and the plan stops being stale.
func (s *Service) Regenerate(ctx context.Context, planID planningdom.PlanID) (shoppingdom.List, error) {
	var result shoppingdom.List
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		plan, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		previous, err := s.previousList(ctx, planID)
		if err != nil {
			return err
		}
		products, err := s.productIndex(ctx)
		if err != nil {
			return err
		}

		saved, err := s.lists.Save(ctx, shoppingdom.Build(plan, products, previous))
		if err != nil {
			return err
		}
		if err := s.plans.MarkShoppingListStale(ctx, planID, false); err != nil {
			return err
		}
		result = saved
		return nil
	})
	if err != nil {
		return shoppingdom.List{}, err
	}
	return result, nil
}

func (s *Service) SetChecked(ctx context.Context, itemID shoppingdom.ItemID, checked bool) error {
	return s.lists.SetChecked(ctx, itemID, checked)
}

func (s *Service) GetItem(ctx context.Context, itemID shoppingdom.ItemID) (shoppingdom.Item, error) {
	return s.lists.GetItem(ctx, itemID)
}

func (s *Service) previousList(ctx context.Context, planID planningdom.PlanID) (shoppingdom.List, error) {
	previous, err := s.lists.Get(ctx, planID)
	if err == nil {
		return previous, nil
	}
	var appError *apperr.Error
	if errors.As(err, &appError) && appError.Code == apperr.CodeNotFound {
		return shoppingdom.List{}, nil
	}
	return shoppingdom.List{}, err
}

func (s *Service) productIndex(ctx context.Context) (map[catalogdom.ProductID]catalogdom.Product, error) {
	products, err := s.products.List(ctx, "")
	if err != nil {
		return nil, err
	}
	index := make(map[catalogdom.ProductID]catalogdom.Product, len(products))
	for i := range products {
		index[products[i].ID] = products[i]
	}
	return index, nil
}
