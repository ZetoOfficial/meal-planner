package planning

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

const (
	defaultPlanDays = 3
	maxNovelty      = 3
	dayDuration     = 24 * time.Hour
)

// EaterTarget is one eater's schedule for the whole plan horizon.
type EaterTarget struct {
	EaterID            eaterdom.ID
	GeneratedMealTypes []mealdom.Type
}

type GeneratePlanCommand struct {
	Label     *string
	StartDate time.Time
	Days      int
	CookDate  *time.Time
	Novelty   int
	Targets   []EaterTarget
}

// PlanResult is the plan together with the quality evaluation that ranked it.
type PlanResult struct {
	Plan      planningdom.Plan
	Snapshots []planningdom.QualitySnapshot
}

// InfeasibleError reports that the solver proved, or ran out of budget proving,
// that no plan satisfies the request. It carries the diagnostics the API needs.
type InfeasibleError struct {
	Failure solver.Failure
}

func (e *InfeasibleError) Error() string {
	return fmt.Sprintf("plan generation failed: %s", e.Failure.Code)
}

type Service struct {
	plans        PlanRepository
	eaters       EaterRepository
	products     ProductRepository
	recipes      RecipeRepository
	planner      Solver
	generator    RecipeGenerator
	transactions TxRunner
	options      Options
}

// Options carries the deterministic planning settings that do not belong to any
// single request: the kitchen equipment the household owns and the rolling
// window weekly rules are projected over.
type Options struct {
	AvailableEquipment []string
	RollingWindowDays  int
}

func NewService(
	plans PlanRepository,
	eaters EaterRepository,
	products ProductRepository,
	recipes RecipeRepository,
	planner Solver,
	generator RecipeGenerator,
	transactions TxRunner,
	options Options,
) *Service {
	options.AvailableEquipment = append([]string(nil), options.AvailableEquipment...)
	return &Service{
		plans:        plans,
		eaters:       eaters,
		products:     products,
		recipes:      recipes,
		planner:      planner,
		generator:    generator,
		transactions: transactions,
		options:      options,
	}
}

// GeneratePlan builds a plan from permanent recipes only and stores it with its
// quality snapshot in one short write transaction. No LLM is involved on the
// happy path: novelty is accepted and recorded, but candidates come from the
// saved library.
//
// Reading the snapshot, searching and asking the LLM all happen without an open
// transaction: a pooled connection must not be held for the search budget plus
// the provider attempts, and the LLM audit rows must survive an outcome that
// stores no plan at all (ARCHITECTURE §8, §11).
func (s *Service) GeneratePlan(ctx context.Context, command GeneratePlanCommand) (PlanResult, error) {
	command, err := normalizeCommand(command)
	if err != nil {
		return PlanResult{}, err
	}

	input, err := s.solverInput(ctx, command)
	if err != nil {
		return PlanResult{}, err
	}
	solved, err := s.planner.BuildPlan(ctx, input)
	if err != nil {
		return PlanResult{}, err
	}
	solved, err = s.withGeneratedCandidates(ctx, command, input, solved)
	if err != nil {
		return PlanResult{}, err
	}
	if solved.Failure != nil {
		return PlanResult{}, &InfeasibleError{Failure: *solved.Failure}
	}

	plan := solved.Plan
	plan.Label = command.Label
	plan.Status = planningdom.PlanStatusDraft
	plan.QualityScore, plan.QualityWarningsCount = planQuality(solved.Quality)
	snapshots := qualitySnapshots(solved.Quality)

	var result PlanResult
	err = s.transactions.WithTx(ctx, func(ctx context.Context) error {
		saved, err := s.plans.Save(ctx, plan)
		if err != nil {
			return err
		}
		if err := s.plans.SaveQualitySnapshots(ctx, saved.ID, snapshots); err != nil {
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

// withGeneratedCandidates gives the solver a second chance with freshly
// generated drafts. A generation failure never replaces the original
// infeasibility: the owner still learns why the saved library was not enough.
func (s *Service) withGeneratedCandidates(
	ctx context.Context,
	command GeneratePlanCommand,
	input solver.Input,
	solved solver.Result,
) (solver.Result, error) {
	if s.generator == nil || !shouldGenerate(solved, command.Novelty) {
		return solved, nil
	}

	drafts, generated := s.tryGenerateCandidates(ctx, command, input)
	if !generated {
		return solved, nil
	}

	input.Recipes = append(append([]recipedom.Template(nil), input.Recipes...), drafts...)
	retried, err := s.planner.BuildPlan(ctx, input)
	if err != nil {
		return solver.Result{}, err
	}
	if retried.Failure != nil && solved.Failure == nil {
		return solved, nil
	}
	return retried, nil
}

// tryGenerateCandidates reports whether usable drafts were produced. A failed
// generation is not an error for the caller: an infeasible plan keeps its
// diagnostics and a feasible one keeps the plan the library already produced.
func (s *Service) tryGenerateCandidates(
	ctx context.Context,
	command GeneratePlanCommand,
	input solver.Input,
) ([]recipedom.Template, bool) {
	drafts, err := s.generateCandidates(ctx, command, input)
	if err != nil {
		return nil, false
	}
	return drafts, true
}

func (s *Service) GetPlan(ctx context.Context, id planningdom.PlanID) (PlanResult, error) {
	plan, err := s.plans.Get(ctx, id)
	if err != nil {
		return PlanResult{}, err
	}
	snapshots, err := s.plans.ListQualitySnapshots(ctx, id, nutrition.SummaryBasisPlanned)
	if err != nil {
		return PlanResult{}, err
	}
	return PlanResult{Plan: plan, Snapshots: snapshots}, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]planningdom.Plan, error) {
	return s.plans.List(ctx)
}

func (s *Service) ArchivePlan(ctx context.Context, id planningdom.PlanID) error {
	return s.plans.Archive(ctx, id)
}

func normalizeCommand(command GeneratePlanCommand) (GeneratePlanCommand, error) {
	if command.StartDate.IsZero() {
		return GeneratePlanCommand{}, apperr.Validation("start_date is required", "start_date")
	}
	command.StartDate = command.StartDate.UTC().Truncate(dayDuration)
	if command.Days == 0 {
		command.Days = defaultPlanDays
	}
	if command.Days < 0 {
		return GeneratePlanCommand{}, apperr.Validation("days must be positive", "days")
	}
	if command.Novelty < 0 || command.Novelty > maxNovelty {
		return GeneratePlanCommand{}, apperr.Validation("novelty_level must be between 0 and 3", "novelty_level")
	}
	if command.CookDate == nil {
		cookDate := command.StartDate
		command.CookDate = &cookDate
	} else {
		cookDate := command.CookDate.UTC().Truncate(dayDuration)
		command.CookDate = &cookDate
	}
	if len(command.Targets) == 0 {
		return GeneratePlanCommand{}, apperr.Validation("at least one eater target is required", "targets")
	}

	seen := make(map[eaterdom.ID]struct{}, len(command.Targets))
	for _, target := range command.Targets {
		if !target.EaterID.Valid() {
			return GeneratePlanCommand{}, apperr.Validation("target eater_id must be positive", "targets")
		}
		if _, exists := seen[target.EaterID]; exists {
			return GeneratePlanCommand{}, apperr.Validation("targets must not repeat an eater", "targets")
		}
		seen[target.EaterID] = struct{}{}
		if err := validateMealTypes(target.GeneratedMealTypes); err != nil {
			return GeneratePlanCommand{}, err
		}
	}
	return command, nil
}

func validateMealTypes(types []mealdom.Type) error {
	if len(types) == 0 {
		return apperr.Validation("generated_meal_types must not be empty", "targets")
	}
	seen := make(map[mealdom.Type]struct{}, len(types))
	for _, mealType := range types {
		if !mealType.Valid() {
			return apperr.Validation("generated_meal_types contains an unknown meal type", string(mealType))
		}
		if _, exists := seen[mealType]; exists {
			return apperr.Validation("generated_meal_types must be unique", string(mealType))
		}
		seen[mealType] = struct{}{}
	}
	return nil
}

func (s *Service) solverInput(ctx context.Context, command GeneratePlanCommand) (solver.Input, error) {
	products, err := s.products.List(ctx, "")
	if err != nil {
		return solver.Input{}, err
	}
	recipes, err := s.recipes.ListPermanentCandidates(ctx)
	if err != nil {
		return solver.Input{}, err
	}

	eaters := make([]solver.EaterInput, 0, len(command.Targets))
	for _, target := range command.Targets {
		eaterInput, err := s.eaterInput(ctx, command, target)
		if err != nil {
			return solver.Input{}, err
		}
		eaters = append(eaters, eaterInput)
	}

	return solver.Input{
		StartDate:          command.StartDate,
		Days:               command.Days,
		CookDate:           *command.CookDate,
		Novelty:            command.Novelty,
		AvailableEquipment: s.options.AvailableEquipment,
		Eaters:             eaters,
		Products:           products,
		Recipes:            recipes,
		RollingWindowDays:  s.options.RollingWindowDays,
	}, nil
}

func (s *Service) eaterInput(
	ctx context.Context,
	command GeneratePlanCommand,
	target EaterTarget,
) (solver.EaterInput, error) {
	profile, err := s.eaters.Get(ctx, target.EaterID)
	if err != nil {
		return solver.EaterInput{}, err
	}
	goals, err := s.eaters.ListGoals(ctx, target.EaterID)
	if err != nil {
		return solver.EaterInput{}, err
	}
	fixedMeals, err := s.eaters.ListFixedMeals(ctx, target.EaterID)
	if err != nil {
		return solver.EaterInput{}, err
	}

	days := make([]solver.EaterDayInput, 0, command.Days)
	for offset := range command.Days {
		date := command.StartDate.AddDate(0, 0, offset)
		goal := eaterdom.CurrentGoalAt(goals, date)
		if goal == nil {
			return solver.EaterInput{}, apperr.Validation(
				fmt.Sprintf("eater %d has no macro goal effective on %s", target.EaterID, date.Format(time.DateOnly)),
				"targets",
			)
		}
		days = append(days, solver.EaterDayInput{
			Date:               date,
			Goal:               *goal,
			GeneratedMealTypes: target.GeneratedMealTypes,
		})
	}

	history, err := s.eaterHistory(ctx, target.EaterID, command.StartDate)
	if err != nil {
		return solver.EaterInput{}, err
	}

	return solver.EaterInput{
		Eater:      profile,
		Days:       days,
		FixedMeals: activeFixedMeals(fixedMeals),
		History:    history,
	}, nil
}

// eaterHistory loads the days that already fall inside the rolling window when
// the plan starts, oldest first. The logged basis wins for a day that has one;
// the planned basis covers the days that were never logged (SPEC §3.2).
func (s *Service) eaterHistory(
	ctx context.Context,
	eaterID eaterdom.ID,
	start time.Time,
) ([]nutrition.DaySummary, error) {
	window := s.options.RollingWindowDays
	if window <= 1 {
		return nil, nil
	}
	from := start.AddDate(0, 0, -(window - 1))
	to := start.AddDate(0, 0, -1)

	planned, err := s.plans.ListEaterDaySummaries(ctx, eaterID, nutrition.SummaryBasisPlanned, from, to)
	if err != nil {
		return nil, err
	}
	actual, err := s.plans.ListEaterDaySummaries(ctx, eaterID, nutrition.SummaryBasisActual, from, to)
	if err != nil {
		return nil, err
	}
	return mergeDayHistory(planned, actual), nil
}

// mergeDayHistory prefers the logged summary of a day over the planned one and
// returns the result oldest first, which is the order rolling rules expect.
func mergeDayHistory(planned, actual []planningdom.QualitySnapshot) []nutrition.DaySummary {
	byDay := make(map[string]nutrition.DaySummary, len(planned)+len(actual))
	for i := range planned {
		byDay[planned[i].Date.UTC().Format(time.DateOnly)] = planned[i].Day
	}
	for i := range actual {
		byDay[actual[i].Date.UTC().Format(time.DateOnly)] = actual[i].Day
	}

	days := make([]string, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Strings(days)

	history := make([]nutrition.DaySummary, 0, len(days))
	for _, day := range days {
		history = append(history, byDay[day])
	}
	return history
}

func activeFixedMeals(fixedMeals []eaterdom.FixedMeal) []eaterdom.FixedMeal {
	active := make([]eaterdom.FixedMeal, 0, len(fixedMeals))
	for i := range fixedMeals {
		if fixedMeals[i].Active {
			active = append(active, fixedMeals[i])
		}
	}
	return active
}

// planQuality reduces the per eater-day evaluations to the plan headline: the
// mean quality score and how many rule outcomes the owner should read.
func planQuality(evaluations []solver.EaterDayQuality) (score *float64, warnings int) {
	if len(evaluations) == 0 {
		return nil, 0
	}
	var total float64
	for i := range evaluations {
		total += evaluations[i].Evaluation.Score
		warnings += evaluations[i].Warnings()
	}
	mean := total / float64(len(evaluations))
	return &mean, warnings
}

func qualitySnapshots(evaluations []solver.EaterDayQuality) []planningdom.QualitySnapshot {
	snapshots := make([]planningdom.QualitySnapshot, 0, len(evaluations))
	for i := range evaluations {
		evaluation := &evaluations[i]
		snapshots = append(snapshots, planningdom.QualitySnapshot{
			EaterID:      evaluation.EaterID,
			Date:         evaluation.Date,
			Basis:        nutrition.SummaryBasisPlanned,
			Day:          evaluation.Day,
			Rolling:      evaluation.Rolling,
			DayRules:     evaluation.DayRules(),
			RollingRules: evaluation.RollingRules(),
		})
	}
	return snapshots
}
