package solver

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

const (
	// noveltyShuffle is the dial level that draws the rotation at random inside
	// the familiar library instead of always taking the heaviest templates.
	noveltyShuffle = 1
	// noveltyRequiresUnseen is the dial level from which a plan must carry a
	// component the household has not eaten for staleAfterDays (DESIGN §7).
	noveltyRequiresUnseen = 2
	staleAfterDays        = 14
	hoursPerDay           = 24
)

// dayPreferences carries the level 5 signals of one eater-day. They are plain
// lookups so the search stays a pure function of its input.
type dayPreferences struct {
	priorUses    map[int64]int
	softExcluded map[catalogdom.ProductID]struct{}
	// shuffleKeys replaces the rotation weight at novelty 1 with a weighted
	// random key that is stable for one plan request.
	shuffleKeys map[recipedom.ID]float64
	// unseen is the set a candidate must draw from while the plan still lacks
	// a novel component; nil when nothing is required.
	unseen map[recipedom.ID]struct{}
	// forceUnseen takes a novel day even when it scores worse on the quality
	// rules; without it a novel day is taken only when it costs no quality.
	forceUnseen bool
	// planned reports whether a variant is already a batch of the plan.
	planned func(componentVariant) bool
}

func (p dayPreferences) requiresUnseen() bool { return len(p.unseen) > 0 }

func (p dayPreferences) hasUnseen(solution daySolution) bool {
	for i := range solution.uses {
		if _, ok := p.unseen[solution.uses[i].variant.recipe.ID]; ok {
			return true
		}
	}
	return false
}

// softExcludedProducts lists the products an eater asked to see less often. A
// soft exclusion never removes a candidate: it only lowers its rank (HC-10).
func softExcludedProducts(eater eaterdom.Eater, products []catalogdom.Product) map[catalogdom.ProductID]struct{} {
	result := make(map[catalogdom.ProductID]struct{})
	for i := range products {
		if matchingExclusion(eater.Exclusions, products[i], eaterdom.ExclusionSeveritySoft) != "" {
			result[products[i].ID] = struct{}{}
		}
	}
	return result
}

// unseenRecipes returns the templates that count as novel for this request.
// Freshly generated drafts win over merely stale ones: novelty 3 asks for the
// unfamiliar dish, not for an old favourite.
func unseenRecipes(input Input) map[recipedom.ID]struct{} {
	if input.Novelty < noveltyRequiresUnseen {
		return nil
	}
	result := make(map[recipedom.ID]struct{})
	for _, id := range input.FreshRecipes {
		result[id] = struct{}{}
	}
	if len(result) > 0 {
		return result
	}
	for i := range input.Recipes {
		id := input.Recipes[i].ID
		lastUsed, used := input.RecipeLastUsed[id]
		if !used || daysBetween(lastUsed, input.StartDate) >= staleAfterDays {
			result[id] = struct{}{}
		}
	}
	return result
}

func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / hoursPerDay)
}

// shuffleKeys draws one weighted random key per template (Efraimidis–Spirakis:
// key = u^(1/weight)). The seed is derived from the request, so the same input
// still yields the same plan while different weeks rotate differently.
func shuffleKeys(input Input) map[recipedom.ID]float64 {
	if input.Novelty != noveltyShuffle {
		return nil
	}
	seed := requestSeed(input)
	keys := make(map[recipedom.ID]float64, len(input.Recipes))
	for i := range input.Recipes {
		recipe := &input.Recipes[i]
		if recipe.Weight <= 0 {
			keys[recipe.ID] = 0
			continue
		}
		keys[recipe.ID] = math.Pow(unitInterval(seed, int64(recipe.ID)), 1/recipe.Weight)
	}
	return keys
}

func requestSeed(input Input) uint64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d/%d", input.StartDate.Unix(), input.Days)
	for i := range input.Eaters {
		_, _ = fmt.Fprintf(hash, "/%d", input.Eaters[i].Eater.ID)
	}
	return hash.Sum64()
}

const (
	hashBits     = 64
	mantissaBits = 53
	// halfStep centres each of the 2^53 buckets, keeping the draw strictly
	// inside (0,1) so a key is never exactly zero or one.
	halfStep = 0.5
)

// unitInterval maps (seed, id) onto (0,1) without touching a global generator.
func unitInterval(seed uint64, id int64) float64 {
	hash := fnv.New64a()
	_, _ = fmt.Fprintf(hash, "%d/%d", seed, id)
	return (float64(hash.Sum64()>>(hashBits-mantissaBits)) + halfStep) / float64(uint64(1)<<mantissaBits)
}
