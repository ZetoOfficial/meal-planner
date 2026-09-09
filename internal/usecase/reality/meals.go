// Package reality closes the loop after planning: what was actually eaten, what
// the owner thought of it, and what the numbers say afterwards.
package reality

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reference"
)

type Service struct {
	meals        MealRepository
	feedback     FeedbackRepository
	analytics    AnalyticsRepository
	reference    reference.Readers
	transactions TxRunner
	clock        Clock
}

func NewService(
	meals MealRepository,
	feedback FeedbackRepository,
	analytics AnalyticsRepository,
	products ProductRepository,
	recipes RecipeRepository,
	eaters EaterRepository,
	transactions TxRunner,
	clock Clock,
) *Service {
	return &Service{
		meals:     meals,
		feedback:  feedback,
		analytics: analytics,
		reference: reference.Readers{
			Products: products,
			Recipes:  recipes,
			Eaters:   eaters,
		},
		transactions: transactions,
		clock:        clock,
	}
}

func (s *Service) ListPlanMeals(
	ctx context.Context,
	planID planningdom.PlanID,
	eaterID *eaterdom.ID,
) ([]planningdom.Meal, error) {
	return s.meals.ListPlanMeals(ctx, planID, eaterID)
}

// LogMeal records what happened to one meal. Actual macros are stored only for
// a modified meal; everything else keeps the plan as the source of truth.
func (s *Service) LogMeal(
	ctx context.Context,
	mealID planningdom.MealID,
	log planningdom.MealLog,
) (planningdom.Meal, error) {
	log.At = s.clock.Now().UTC()
	if err := log.Validate(); err != nil {
		return planningdom.Meal{}, apperr.Validation("meal log is not valid", err.Error())
	}
	return s.meals.LogMeal(ctx, mealID, log)
}
