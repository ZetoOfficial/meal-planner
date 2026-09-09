package planning

import (
	"context"
	"fmt"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

// noveltyForcesGeneration is the dial level at which the owner explicitly asks
// for a brand new template even though the library already has a feasible plan.
const noveltyForcesGeneration = 3

// shouldGenerate keeps the LLM off the happy path: it runs only when the saved
// library produced no feasible plan, or when novelty 3 explicitly asks for one.
func shouldGenerate(result solver.Result, novelty int) bool {
	return result.Failure != nil || novelty >= noveltyForcesGeneration
}

// generateCandidates asks for new templates, validates each one against the
// same catalogue the solver sees, and stores the survivors as drafts.
func (s *Service) generateCandidates(
	ctx context.Context,
	command GeneratePlanCommand,
	input solver.Input,
) ([]recipedom.Template, error) {
	roleTags, err := s.products.ListRoleTags(ctx)
	if err != nil {
		return nil, err
	}

	candidates, err := s.generator.GenerateCandidates(ctx, RecipeCandidateRequest{
		Days:       command.Days,
		Count:      len(command.Targets),
		Novelty:    command.Novelty,
		MealTypes:  requestedMealTypes(command),
		Equipment:  s.options.AvailableEquipment,
		Allergens:  allergenNames(input.Eaters),
		Exclusions: hardExclusionLabels(input.Eaters),
		RoleTags:   roleTags,
		Products:   input.Products,
	})
	if err != nil {
		return nil, err
	}

	usable := make([]recipedom.Template, 0, len(candidates))
	for i := range candidates {
		if err := validateCandidate(candidates[i], input, s.options.AvailableEquipment); err != nil {
			continue
		}
		usable = append(usable, candidates[i])
	}
	if len(usable) == 0 {
		return nil, apperr.Validation("no generated recipe candidate survived validation")
	}

	// The drafts commit on their own, after the provider call has returned:
	// they are a library addition in their own right and must not be rolled
	// back with a plan that later turns out to be unstorable.
	accepted := make([]recipedom.Template, 0, len(usable))
	err = s.transactions.WithTx(ctx, func(ctx context.Context) error {
		accepted = accepted[:0]
		for i := range usable {
			stored, err := s.storeDraft(ctx, usable[i])
			if err != nil {
				return err
			}
			accepted = append(accepted, stored)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accepted, nil
}

// storeDraft persists an accepted candidate as a draft. Drafts are promoted to
// permanent only after the dish has actually been cooked.
func (s *Service) storeDraft(ctx context.Context, candidate recipedom.Template) (recipedom.Template, error) {
	resolved, err := s.recipes.ResolveRoleTags(ctx, candidate.RoleCodes())
	if err != nil {
		return recipedom.Template{}, err
	}
	candidate.Status = recipedom.StatusDraft
	candidate.Source = recipedom.SourceLLM
	if err := candidate.Validate(resolved); err != nil {
		return recipedom.Template{}, apperr.Validation("generated recipe candidate is invalid", err.Error())
	}
	return s.recipes.Create(ctx, candidate, resolved)
}

// validateCandidate rejects templates the household cannot actually cook: a
// slot no stocked product can fill, equipment nobody owns, or a batch that will
// not keep for the plan horizon.
func validateCandidate(candidate recipedom.Template, input solver.Input, equipment []string) error {
	if input.Days > 1 && !candidate.Storage.SupportsMultiDay() {
		return fmt.Errorf("storage %s does not cover %d days", candidate.Storage, input.Days)
	}
	available := make(map[string]struct{}, len(equipment))
	for _, item := range equipment {
		available[item] = struct{}{}
	}
	for _, item := range candidate.Equipment {
		if _, ok := available[item]; !ok {
			return fmt.Errorf("equipment %s is unavailable", item)
		}
	}
	for _, slot := range candidate.Slots {
		if slot.Optional {
			continue
		}
		if !roleIsStocked(slot.RoleTagCode, input.Products) {
			return fmt.Errorf("no stocked product fills role %s", slot.RoleTagCode)
		}
	}
	return nil
}

func roleIsStocked(code string, products []catalogdom.Product) bool {
	for i := range products {
		for _, role := range products[i].Roles {
			if role.Code == code {
				return true
			}
		}
	}
	return false
}

func requestedMealTypes(command GeneratePlanCommand) []mealdom.Type {
	seen := make(map[mealdom.Type]struct{})
	result := make([]mealdom.Type, 0)
	for _, target := range command.Targets {
		for _, mealType := range target.GeneratedMealTypes {
			if _, exists := seen[mealType]; exists {
				continue
			}
			seen[mealType] = struct{}{}
			result = append(result, mealType)
		}
	}
	return result
}

func allergenNames(eaters []solver.EaterInput) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for i := range eaters {
		for _, allergen := range eaters[i].Eater.Allergens {
			if _, exists := seen[string(allergen)]; exists {
				continue
			}
			seen[string(allergen)] = struct{}{}
			result = append(result, string(allergen))
		}
	}
	return result
}

func hardExclusionLabels(eaters []solver.EaterInput) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for i := range eaters {
		for _, exclusion := range eaters[i].Eater.Exclusions {
			if exclusion.Severity != eaterdom.ExclusionSeverityHard {
				continue
			}
			label := fmt.Sprintf("%s: %s", exclusion.TargetKind, exclusion.TargetValue)
			if _, exists := seen[label]; exists {
				continue
			}
			seen[label] = struct{}{}
			result = append(result, label)
		}
	}
	return result
}
