package repositories

import (
	"errors"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	cataloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/catalog"
	eateruc "github.com/ZetoOfficial/meal-planner/internal/usecase/eater"
	feedbackuc "github.com/ZetoOfficial/meal-planner/internal/usecase/feedback"
	mealloguc "github.com/ZetoOfficial/meal-planner/internal/usecase/meal_log"
	planninguc "github.com/ZetoOfficial/meal-planner/internal/usecase/planning"
	recipeuc "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

var ErrNotImplemented = errors.New("postgres repository method not implemented")

type Repositories struct {
	Products cataloguc.ProductRepository
	Eaters   eateruc.EaterRepository
	Recipes  recipeuc.RecipeRepository
	Plans    planninguc.PlanRepository
	Meals    mealloguc.MealRepository
	Feedback feedbackuc.FeedbackRepository
}

func New(db sqlc.DBTX) Repositories {
	q := sqlc.New(db)
	return Repositories{
		Products: NewProductRepository(db),
		Eaters:   NewEaterRepository(),
		Recipes:  NewRecipeRepository(q),
		Plans:    NewPlanRepository(q),
		Meals:    NewMealRepository(q),
		Feedback: NewFeedbackRepository(q),
	}
}

func (r Repositories) Planning() planninguc.Repositories {
	return planninguc.Repositories{
		Products: r.Products,
		Eaters:   r.Eaters,
		Recipes:  r.Recipes,
		Plans:    r.Plans,
	}
}
