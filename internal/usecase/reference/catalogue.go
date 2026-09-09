// Package reference loads the reference data a stored plan needs to be read
// back as quality input. Planning and reality both re-evaluate stored plans and
// both need exactly this set, so the loader lives in one place.
package reference

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

type ProductReader interface {
	List(ctx context.Context, roleCode string) ([]catalogdom.Product, error)
}

type RecipeReader interface {
	List(ctx context.Context, status *recipedom.Status) ([]recipedom.Template, error)
}

type EaterReader interface {
	Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error)
}

// Readers groups the three narrow ports the loader needs.
type Readers struct {
	Products ProductReader
	Recipes  RecipeReader
	Eaters   EaterReader
}

// LoadCatalogue reads the products and templates once and the profile and fixed
// meals of every eater the plans mention, so a stored plan can be rebuilt into
// quality input without the numbers drifting between use cases.
func LoadCatalogue(
	ctx context.Context,
	readers Readers,
	plans []planningdom.Plan,
) (quality.Catalogue, error) {
	products, err := readers.Products.List(ctx, "")
	if err != nil {
		return quality.Catalogue{}, err
	}
	templates, err := readers.Recipes.List(ctx, nil)
	if err != nil {
		return quality.Catalogue{}, err
	}

	catalogue := quality.Catalogue{
		Products:   make(map[catalogdom.ProductID]catalogdom.Product, len(products)),
		Templates:  make(map[recipedom.ID]recipedom.Template, len(templates)),
		Eaters:     make(map[eaterdom.ID]eaterdom.Eater),
		FixedMeals: make(map[eaterdom.FixedMealID]eaterdom.FixedMeal),
	}
	for i := range products {
		catalogue.Products[products[i].ID] = products[i]
	}
	for i := range templates {
		catalogue.Templates[templates[i].ID] = templates[i]
	}

	for i := range plans {
		for j := range plans[i].EaterDays {
			eaterID := plans[i].EaterDays[j].EaterID
			if _, loaded := catalogue.Eaters[eaterID]; loaded {
				continue
			}
			eater, err := readers.Eaters.Get(ctx, eaterID)
			if err != nil {
				return quality.Catalogue{}, err
			}
			catalogue.Eaters[eaterID] = eater

			fixedMeals, err := readers.Eaters.ListFixedMeals(ctx, eaterID)
			if err != nil {
				return quality.Catalogue{}, err
			}
			for k := range fixedMeals {
				catalogue.FixedMeals[fixedMeals[k].ID] = fixedMeals[k]
			}
		}
	}
	return catalogue, nil
}
