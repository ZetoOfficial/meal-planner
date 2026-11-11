package planning

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
	"github.com/ZetoOfficial/meal-planner/internal/swap"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reference"
)

// SwapCommand names one swap. The level decides which fields matter:
//
//   - fill: SlotFillID and NewProductID replace the product of one slot;
//   - recipe: PlanRecipeID and NewTemplateID replace what one batch cooks;
//   - rotation: nothing else — the whole plan is rebuilt around other recipes.
type SwapCommand struct {
	Level         planningdom.SwapLevel
	SlotFillID    planningdom.SlotFillID
	NewProductID  catalogdom.ProductID
	PlanRecipeID  planningdom.PlanRecipeID
	NewTemplateID recipedom.ID
}

// computedSwap is a preview together with what applying it needs.
type computedSwap struct {
	preview     swap.Preview
	recipeIndex int
	// rotation holds the solver's own evaluation of a rebuilt plan.
	rotation []solver.EaterDayQuality
}

// PreviewSwap computes the blast radius without changing anything.
func (s *Service) PreviewSwap(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
) (swap.Preview, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return swap.Preview{}, err
	}
	computed, err := s.computeSwap(ctx, plan, command)
	if err != nil {
		return swap.Preview{}, err
	}
	return computed.preview, nil
}

func (s *Service) computeSwap(ctx context.Context, plan planningdom.Plan, command SwapCommand) (computedSwap, error) {
	if plan.Status == planningdom.PlanStatusArchived {
		return computedSwap{}, apperr.Conflict("an archived plan cannot be changed")
	}
	input, err := s.swapInput(ctx, plan)
	if err != nil {
		return computedSwap{}, err
	}

	switch command.Level {
	case planningdom.SwapFill:
		preview, err := swap.Compute(input, swap.Request{
			SlotFillID:   command.SlotFillID,
			NewProductID: command.NewProductID,
		})
		if err != nil {
			return computedSwap{}, swapRequestError("swap cannot be applied", err)
		}
		return computedSwap{preview: preview}, nil
	case planningdom.SwapRecipe:
		recipeIndex := recipeIndexOf(plan, command.PlanRecipeID)
		if batchHasLoggedMeals(plan, recipeIndex) {
			return computedSwap{}, apperr.Conflict("a batch with logged meals cannot be swapped for another recipe")
		}
		preview, err := swap.ComputeRecipe(input, swap.RecipeRequest{
			PlanRecipeID:  command.PlanRecipeID,
			NewTemplateID: command.NewTemplateID,
		})
		if err != nil {
			return computedSwap{}, swapRequestError("swap cannot be applied", err)
		}
		return computedSwap{preview: preview, recipeIndex: recipeIndex}, nil
	case planningdom.SwapRotation:
		return s.computeRotation(ctx, plan, input)
	default:
		return computedSwap{}, apperr.Validation("level must be fill, recipe or rotation", "level")
	}
}

func recipeIndexOf(plan planningdom.Plan, id planningdom.PlanRecipeID) int {
	for i := range plan.Recipes {
		if plan.Recipes[i].ID == id {
			return i
		}
	}
	return -1
}

// batchHasLoggedMeals reports whether anything was already eaten (or skipped)
// from the batch: replacing its recipe would rewrite what those meals record.
func batchHasLoggedMeals(plan planningdom.Plan, recipeIndex int) bool {
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		if !logged(meal.LogStatus) {
			continue
		}
		for _, component := range meal.Components {
			if component.PlanRecipeIndex == recipeIndex {
				return true
			}
		}
	}
	return false
}

func logged(status planningdom.MealLogStatus) bool {
	return status != "" && status != planningdom.MealLogStatusPlanned
}

// sameBatches reports whether two plans cook exactly the same batches: the
// same templates with the same products in their slots.
func sameBatches(left, right planningdom.Plan) bool {
	if len(left.Recipes) != len(right.Recipes) {
		return false
	}
	counts := make(map[string]int, len(left.Recipes))
	for i := range left.Recipes {
		counts[batchSignature(left.Recipes[i])]++
	}
	for i := range right.Recipes {
		signature := batchSignature(right.Recipes[i])
		if counts[signature] == 0 {
			return false
		}
		counts[signature]--
	}
	return true
}

func batchSignature(recipe planningdom.PlanRecipe) string {
	fills := make([]string, 0, len(recipe.Fills))
	for _, fill := range recipe.Fills {
		fills = append(fills, fmt.Sprintf("%d:%d", fill.SlotID, fill.ProductID))
	}
	sort.Strings(fills)
	return fmt.Sprintf("%d|%s", recipe.TemplateID, strings.Join(fills, ","))
}

// computeRotation rebuilds the plan for the same eaters and days. It first
// looks for a plan that uses none of the current templates — that is what a new
// rotation means — and only falls back to the whole library when the rest of
// it cannot feed the household. The LLM is not asked: a rotation is a
// deterministic reshuffle of what the library already holds.
func (s *Service) computeRotation(
	ctx context.Context,
	plan planningdom.Plan,
	input swap.Input,
) (computedSwap, error) {
	for i := range plan.Meals {
		if logged(plan.Meals[i].LogStatus) {
			return computedSwap{}, apperr.Conflict("a plan with logged meals cannot be rotated: generate a new plan instead")
		}
	}

	solverInput, err := s.solverInput(ctx, rotationCommand(plan))
	if err != nil {
		return computedSwap{}, err
	}
	current := make(map[recipedom.ID]struct{}, len(plan.Recipes))
	for i := range plan.Recipes {
		current[plan.Recipes[i].TemplateID] = struct{}{}
	}
	fresh := solverInput
	fresh.Recipes = make([]recipedom.Template, 0, len(solverInput.Recipes))
	for i := range solverInput.Recipes {
		if _, used := current[solverInput.Recipes[i].ID]; !used {
			fresh.Recipes = append(fresh.Recipes, solverInput.Recipes[i])
		}
	}

	solved, err := s.planner.BuildPlan(ctx, fresh)
	if err != nil {
		return computedSwap{}, err
	}
	if solved.Failure != nil {
		if solved, err = s.planner.BuildPlan(ctx, solverInput); err != nil {
			return computedSwap{}, err
		}
	}
	if solved.Failure != nil {
		return computedSwap{}, apperr.Validation("no other rotation fits the plan", failureDetails(*solved.Failure)...)
	}

	if sameBatches(plan, solved.Plan) {
		// The solver is deterministic: with nothing else to cook it rebuilds
		// the plan it was asked to replace.
		return computedSwap{}, apperr.Validation("no other rotation fits the plan: the library offers no other feasible set of recipes")
	}

	replacement := solved.Plan
	replacement.Label = plan.Label
	replacement.Status = planningdom.PlanStatusDraft
	replacement.QualityScore, replacement.QualityWarningsCount = planQuality(solved.Quality)
	preview, err := swap.ComputeRotation(input, replacement)
	if err != nil {
		return computedSwap{}, err
	}
	return computedSwap{preview: preview, rotation: solved.Quality}, nil
}

// rotationCommand reads the request a stored plan was generated from. Each
// eater's schedule is the one saved for their first day.
func rotationCommand(plan planningdom.Plan) GeneratePlanCommand {
	cookDate := plan.CookDate
	command := GeneratePlanCommand{
		Label:     plan.Label,
		StartDate: plan.StartDate,
		Days:      plan.Days,
		CookDate:  &cookDate,
		Novelty:   plan.Novelty,
	}
	seen := make(map[eaterdom.ID]struct{})
	for i := range plan.EaterDays {
		day := &plan.EaterDays[i]
		if _, exists := seen[day.EaterID]; exists {
			continue
		}
		seen[day.EaterID] = struct{}{}
		command.Targets = append(command.Targets, EaterTarget{
			EaterID:            day.EaterID,
			GeneratedMealTypes: day.GeneratedMealTypes,
		})
	}
	return command
}

func failureDetails(failure solver.Failure) []string {
	diagnostics := failure.Diagnostics()
	details := make([]string, 0, len(diagnostics))
	for i := range diagnostics {
		details = append(details, diagnostics[i].Message)
	}
	return details
}

// ApplySwap writes a confirmed swap and re-snapshots the plan's quality, so the
// stored explanation keeps matching the stored plan. Every swap needs an
// explicit confirmation: the preview is the price, the confirmation accepts it.
//
// A fill or recipe swap changes the plan in place. A rotation stores the new
// plan and archives the old one, so the returned plan has a new id.
func (s *Service) ApplySwap(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
	confirmed bool,
) (PlanResult, error) {
	if command.Level == planningdom.SwapRotation {
		return s.applyRotation(ctx, planID, command, confirmed)
	}

	var result PlanResult
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		plan, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		computed, err := s.computeSwap(ctx, plan, command)
		if err != nil {
			return err
		}
		if !confirmed {
			return &UnconfirmedSwapError{Preview: computed.preview}
		}

		if command.Level == planningdom.SwapRecipe {
			err = s.plans.ReplacePlanRecipe(ctx, planID, computed.preview.Plan, computed.recipeIndex)
		} else {
			err = s.applyFill(ctx, planID, command, computed.preview.Plan)
		}
		if err != nil {
			return err
		}

		snapshots := swapSnapshots(computed.preview)
		if err := s.plans.SaveQualitySnapshots(ctx, planID, snapshots); err != nil {
			return err
		}
		score, warnings := snapshotQuality(computed.preview.EaterDays)
		if err := s.plans.UpdatePlanQuality(ctx, planID, score, warnings); err != nil {
			return err
		}
		stored, err := s.plans.Get(ctx, planID)
		if err != nil {
			return err
		}
		result = PlanResult{Plan: stored, Snapshots: snapshots}
		return nil
	})
	if err != nil {
		return PlanResult{}, err
	}
	return result, nil
}

func (s *Service) applyFill(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
	swapped planningdom.Plan,
) error {
	if err := s.plans.SwapSlotFill(ctx, planID, command.SlotFillID, command.NewProductID); err != nil {
		return err
	}
	return s.plans.UpdateMealMacros(ctx, swapped)
}

// applyRotation runs the search outside any transaction, like plan generation
// does, and then swaps the plans in one short write.
func (s *Service) applyRotation(
	ctx context.Context,
	planID planningdom.PlanID,
	command SwapCommand,
	confirmed bool,
) (PlanResult, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return PlanResult{}, err
	}
	computed, err := s.computeSwap(ctx, plan, command)
	if err != nil {
		return PlanResult{}, err
	}
	if !confirmed {
		return PlanResult{}, &UnconfirmedSwapError{Preview: computed.preview}
	}

	snapshots := qualitySnapshots(computed.rotation)
	var result PlanResult
	err = s.transactions.WithTx(ctx, func(ctx context.Context) error {
		saved, err := s.plans.Save(ctx, computed.preview.Plan)
		if err != nil {
			return err
		}
		if err := s.plans.SaveQualitySnapshots(ctx, saved.ID, snapshots); err != nil {
			return err
		}
		if err := s.plans.Archive(ctx, planID); err != nil {
			return err
		}
		result = PlanResult{Plan: saved, Snapshots: snapshots}
		return nil
	})
	if err != nil {
		return PlanResult{}, err
	}
	return result, nil
}

// snapshotQuality reduces a re-evaluated plan to its headline, the same way
// plan generation does: the mean day score and the outcomes worth reading.
func snapshotQuality(days []swap.DaySnapshot) (score *float64, warnings int) {
	if len(days) == 0 {
		return nil, 0
	}
	var total float64
	for i := range days {
		total += days[i].Score
		for _, rules := range [][]nutrition.RuleEvaluationResult{days[i].DayRules, days[i].RollingRules} {
			for _, rule := range rules {
				if rule.Status.NeedsAttention() {
					warnings++
				}
			}
		}
	}
	mean := total / float64(len(days))
	return &mean, warnings
}

// swapRequestError reports only the rejections that describe a bad request as
// validation failures. A plan that no longer matches the catalogue is corrupt
// data: turning it into 422 would blame the owner for a storage problem.
func swapRequestError(message string, err error) error {
	switch {
	case errors.Is(err, swap.ErrUnknownFill),
		errors.Is(err, swap.ErrUnknownProduct),
		errors.Is(err, swap.ErrUnknownRecipe),
		errors.Is(err, swap.ErrUnknownTemplate),
		errors.Is(err, swap.ErrNonPositiveQuantity):
		return apperr.Validation(message, err.Error())
	case errors.As(err, new(*swap.IncompatibleError)):
		return apperr.Validation(message, err.Error())
	default:
		return err
	}
}

// UnconfirmedSwapError carries the preview the owner still has to accept.
type UnconfirmedSwapError struct {
	Preview swap.Preview
}

func (e *UnconfirmedSwapError) Error() string {
	return "swap needs explicit confirmation"
}

func swapSnapshots(preview swap.Preview) []planningdom.QualitySnapshot {
	snapshots := make([]planningdom.QualitySnapshot, 0, len(preview.EaterDays))
	for i := range preview.EaterDays {
		day := &preview.EaterDays[i]
		snapshots = append(snapshots, planningdom.QualitySnapshot{
			EaterID:      day.EaterID,
			Date:         day.Date,
			Basis:        day.Basis,
			Day:          day.Day,
			Rolling:      day.Rolling,
			DayRules:     day.DayRules,
			RollingRules: day.RollingRules,
		})
	}
	return snapshots
}

// swapInput assembles the catalogue and the rolling-window history a stored
// plan needs to be re-evaluated the same way it was built.
func (s *Service) swapInput(ctx context.Context, plan planningdom.Plan) (swap.Input, error) {
	catalogue, err := reference.LoadCatalogue(ctx, reference.Readers{
		Products: s.products,
		Recipes:  s.recipes,
		Eaters:   s.eaters,
	}, []planningdom.Plan{plan})
	if err != nil {
		return swap.Input{}, err
	}
	history := make(map[eaterdom.ID][]nutrition.DaySummary, len(plan.EaterDays))
	for i := range plan.EaterDays {
		eaterID := plan.EaterDays[i].EaterID
		if _, loaded := history[eaterID]; loaded {
			continue
		}
		days, err := s.eaterHistory(ctx, eaterID, plan.StartDate)
		if err != nil {
			return swap.Input{}, err
		}
		history[eaterID] = days
	}
	goals, err := s.planGoals(ctx, plan)
	if err != nil {
		return swap.Input{}, err
	}
	return swap.Input{
		Goals:              goals,
		Plan:               plan,
		Catalogue:          catalogue,
		Ruleset:            quality.DefaultRuleset(),
		RollingWindowDays:  s.options.RollingWindowDays,
		History:            history,
		AvailableEquipment: s.options.AvailableEquipment,
	}, nil
}
