package reality

import (
	"context"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type AnalyticsRepository interface {
	ListDaySummaries(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]nutrition.DaySummary, error)
	ListRollingSummaries(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]nutrition.RollingSummary, error)
	ListRuleEvaluations(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]RuleEvaluation, error)
	ListTopRecipes(ctx context.Context, limit int) ([]TopRecipe, error)
	ListProductFrequency(ctx context.Context, eaterID eaterdom.ID, from, to time.Time, limit int) ([]ProductFrequency, error)
}

// ProductFrequency reports how many of an eater's meals used a product and how
// many input grams those meals accounted for.
type ProductFrequency struct {
	ProductID  catalogdom.ProductID
	Brand      string
	Name       string
	MealsCount int
	TotalQtyG  float64
}

// TopRecipe reports how often a template has actually been cooked and how often
// plans reached for it.
type TopRecipe struct {
	RecipeID    recipedom.ID
	Name        string
	CookedCount int
	PlanUsage   int
}
