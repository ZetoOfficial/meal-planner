package reality

import (
	"context"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type AnalyticsRepository interface {
	ListDaySummaries(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]nutrition.DaySummary, error)
	ListRollingSummaries(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]nutrition.RollingSummary, error)
	ListRuleEvaluations(ctx context.Context, eaterID eaterdom.ID, basis nutrition.SummaryBasis, from, to time.Time) ([]RuleEvaluation, error)
	ListTopRecipes(ctx context.Context, limit int) ([]TopRecipe, error)
}

// TopRecipe reports how often a template has actually been cooked and how often
// plans reached for it.
type TopRecipe struct {
	RecipeID    recipedom.ID
	Name        string
	CookedCount int
	PlanUsage   int
}
