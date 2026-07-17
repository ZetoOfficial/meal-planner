package eater

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
)

const (
	maxNumeric6_1 = 99_999.9
	maxNumeric7_1 = 999_999.9
	maxNumeric8_1 = 9_999_999.9
	dayDuration   = 24 * time.Hour
)

type Service struct {
	eaters       EaterRepository
	transactions TxRunner
	clock        Clock
}

func NewService(eaters EaterRepository, transactions TxRunner, clock Clock) *Service {
	if transactions == nil {
		panic("eater transaction runner is required")
	}
	return &Service{eaters: eaters, transactions: transactions, clock: clock}
}

type MacroGoalCommand struct {
	EffectiveFrom time.Time
	Range         eaterdom.MacroRange
	Note          *string
}

type CreateEaterCommand struct {
	Name      string
	Sex       eaterdom.Sex
	Allergens []eaterdom.Allergen
	Goal      MacroGoalCommand
}

type UpdateEaterCommand = CreateEaterCommand

func (s *Service) CreateEater(ctx context.Context, command CreateEaterCommand) (eaterdom.Eater, error) {
	profile, goal, err := profileFromCommand(command, s.clock.Now())
	if err != nil {
		return eaterdom.Eater{}, err
	}
	var created eaterdom.Eater
	err = s.transactions.WithTx(ctx, func(ctx context.Context) error {
		created, err = s.eaters.Create(ctx, profile, goal)
		return err
	})
	if err != nil {
		return eaterdom.Eater{}, err
	}
	return created, nil
}

func (s *Service) ListEaters(ctx context.Context) ([]eaterdom.Eater, error) {
	return s.eaters.List(ctx)
}

func (s *Service) GetEater(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error) {
	if err := validateEaterID(id); err != nil {
		return eaterdom.Eater{}, err
	}
	return s.eaters.Get(ctx, id)
}

func (s *Service) UpdateEater(ctx context.Context, id eaterdom.ID, command UpdateEaterCommand) (eaterdom.Eater, error) {
	if err := validateEaterID(id); err != nil {
		return eaterdom.Eater{}, err
	}
	goalEffectiveFromDefaulted := command.Goal.EffectiveFrom.IsZero()
	profile, goal, err := profileFromCommand(command, s.clock.Now())
	if err != nil {
		return eaterdom.Eater{}, err
	}
	profile.ID, goal.EaterID = id, id
	var updated eaterdom.Eater
	err = s.transactions.WithTx(ctx, func(ctx context.Context) error {
		if goalEffectiveFromDefaulted {
			goals, err := s.eaters.ListGoals(ctx, id)
			if err != nil {
				return err
			}
			// PUT has no effective_from. Never overwrite today's immutable goal:
			// when it already exists, schedule the replacement for the first free
			// future date so the current goal remains historically correct today.
			goal.EffectiveFrom = firstAvailableGoalDate(goals, goal.EffectiveFrom)
		}
		updated, err = s.eaters.Update(ctx, profile, goal)
		return err
	})
	if err != nil {
		return eaterdom.Eater{}, err
	}
	return updated, nil
}

func firstAvailableGoalDate(goals []eaterdom.MacroGoal, candidate time.Time) time.Time {
	candidate = candidate.UTC().Truncate(dayDuration)
	for {
		occupied := false
		for _, goal := range goals {
			if goal.EffectiveFrom.UTC().Truncate(dayDuration).Equal(candidate) {
				occupied = true
				break
			}
		}
		if !occupied {
			return candidate
		}
		candidate = candidate.AddDate(0, 0, 1)
	}
}

func (s *Service) ListGoals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	if err := validateEaterID(eaterID); err != nil {
		return nil, err
	}
	return s.eaters.ListGoals(ctx, eaterID)
}

func (s *Service) CreateGoal(ctx context.Context, eaterID eaterdom.ID, command MacroGoalCommand) (eaterdom.MacroGoal, error) {
	if err := validateEaterID(eaterID); err != nil {
		return eaterdom.MacroGoal{}, err
	}
	goal, err := goalFromCommand(eaterID, command, s.clock.Now())
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return s.eaters.CreateGoal(ctx, goal)
}

type CreateFixedMealCommand struct {
	MealType           eaterdom.MealType
	Label              string
	Macros             eaterdom.Macros
	Quality            eaterdom.FixedMealQuality
	IncludeInAnalytics *bool
	IncludeInWarnings  *bool
}

func (s *Service) ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	if err := validateEaterID(eaterID); err != nil {
		return nil, err
	}
	return s.eaters.ListFixedMeals(ctx, eaterID)
}

func (s *Service) CreateFixedMeal(ctx context.Context, eaterID eaterdom.ID, command CreateFixedMealCommand) (eaterdom.FixedMeal, error) {
	if err := validateEaterID(eaterID); err != nil {
		return eaterdom.FixedMeal{}, err
	}
	quality := command.Quality
	quality.IncludeInAnalytics = optionalBool(command.IncludeInAnalytics, true)
	quality.IncludeInWarnings = optionalBool(command.IncludeInWarnings, true)
	meal := eaterdom.FixedMeal{EaterID: eaterID, MealType: command.MealType, Label: strings.TrimSpace(command.Label), Macros: command.Macros, Quality: quality, Active: true}
	if err := validateFixedMeal(&meal); err != nil {
		return eaterdom.FixedMeal{}, err
	}
	return s.eaters.CreateFixedMeal(ctx, meal)
}

type CreateExclusionCommand struct {
	TargetKind  eaterdom.ExclusionTarget
	TargetValue string
	Severity    eaterdom.ExclusionSeverity
	Note        *string
}

func (s *Service) ListExclusions(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.Exclusion, error) {
	if err := validateEaterID(eaterID); err != nil {
		return nil, err
	}
	return s.eaters.ListExclusions(ctx, eaterID)
}

func (s *Service) CreateExclusion(ctx context.Context, eaterID eaterdom.ID, command CreateExclusionCommand) (eaterdom.Exclusion, error) {
	if err := validateEaterID(eaterID); err != nil {
		return eaterdom.Exclusion{}, err
	}
	exclusion := eaterdom.Exclusion{EaterID: eaterID, TargetKind: command.TargetKind, TargetValue: strings.TrimSpace(command.TargetValue), Severity: command.Severity, Note: command.Note}
	if exclusion.Severity == "" {
		exclusion.Severity = eaterdom.ExclusionSeverityHard
	}
	if err := s.validateExclusion(ctx, exclusion); err != nil {
		return eaterdom.Exclusion{}, err
	}
	return s.eaters.CreateExclusion(ctx, exclusion)
}

func (s *Service) DeleteExclusion(ctx context.Context, id eaterdom.ExclusionID) error {
	if id <= 0 {
		return apperr.Validation("exclusion id must be positive")
	}
	return s.eaters.DeleteExclusion(ctx, id)
}

func profileFromCommand(command CreateEaterCommand, now time.Time) (eaterdom.Eater, eaterdom.MacroGoal, error) {
	profile := eaterdom.Eater{Name: strings.TrimSpace(command.Name), Sex: command.Sex}
	if profile.Name == "" {
		return eaterdom.Eater{}, eaterdom.MacroGoal{}, apperr.Validation("eater name must not be blank")
	}
	if profile.Sex == "" {
		profile.Sex = eaterdom.SexUnspecified
	}
	if !validSex(profile.Sex) {
		return eaterdom.Eater{}, eaterdom.MacroGoal{}, apperr.Validation("sex is invalid")
	}
	allergens, err := normalizeAllergens(command.Allergens)
	if err != nil {
		return eaterdom.Eater{}, eaterdom.MacroGoal{}, err
	}
	profile.Allergens = allergens
	goal, err := goalFromCommand(0, command.Goal, now)
	if err != nil {
		return eaterdom.Eater{}, eaterdom.MacroGoal{}, err
	}
	return profile, goal, nil
}

func goalFromCommand(eaterID eaterdom.ID, command MacroGoalCommand, now time.Time) (eaterdom.MacroGoal, error) {
	goal := eaterdom.MacroGoal{EaterID: eaterID, EffectiveFrom: command.EffectiveFrom, Range: command.Range, Note: command.Note}
	if goal.EffectiveFrom.IsZero() {
		goal.EffectiveFrom = now.UTC().Truncate(dayDuration)
	}
	if err := validateMacroRange(goal.Range); err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return goal, nil
}

func validateEaterID(id eaterdom.ID) error {
	if id <= 0 {
		return apperr.Validation("eater id must be positive")
	}
	return nil
}

func validateMacroRange(value eaterdom.MacroRange) error {
	for name, limits := range map[string][2]float64{
		"protein": {value.ProteinMinG, value.ProteinMaxG},
		"fat":     {value.FatMinG, value.FatMaxG},
		"carb":    {value.CarbMinG, value.CarbMaxG},
	} {
		if err := validateNumeric6_1(name+"_min_g", limits[0]); err != nil {
			return err
		}
		if err := validateNumeric6_1(name+"_max_g", limits[1]); err != nil {
			return err
		}
		if limits[0] < 0 || limits[0] > limits[1] {
			return apperr.Validation(name + " range must be nonnegative and ordered")
		}
	}
	if !validKcal(value.KcalMin) || !validKcal(value.KcalMax) || value.KcalMin > value.KcalMax {
		return apperr.Validation("kcal range must be nonnegative and ordered")
	}
	return nil
}

func validateFixedMeal(meal *eaterdom.FixedMeal) error {
	if !validMealType(meal.MealType) || meal.Label == "" {
		return apperr.Validation("fixed meal type and label are required")
	}
	for name, value := range map[string]float64{
		"protein_g": meal.Macros.ProteinG,
		"fat_g":     meal.Macros.FatG,
		"carb_g":    meal.Macros.CarbG,
	} {
		if err := validateNumeric6_1("fixed meal "+name, value); err != nil {
			return err
		}
	}
	if !validKcal(meal.Macros.Kcal) {
		return apperr.Validation("fixed meal kcal must be a nonnegative int32")
	}
	for name, value := range map[string]struct {
		value      *float64
		upperBound float64
	}{
		"saturated_fat_g": {meal.Quality.SaturatedFatG, maxNumeric6_1}, "trans_fat_g": {meal.Quality.TransFatG, maxNumeric6_1},
		"fiber_g": {meal.Quality.FiberG, maxNumeric6_1}, "sodium_mg": {meal.Quality.SodiumMg, maxNumeric8_1},
		"added_sugars_g": {meal.Quality.AddedSugarsG, maxNumeric6_1}, "free_sugars_g": {meal.Quality.FreeSugarsG, maxNumeric6_1},
		"processed_meat_g": {meal.Quality.ProcessedMeatG, maxNumeric7_1}, "red_meat_g": {meal.Quality.RedMeatG, maxNumeric7_1},
		"whole_grain_g_equiv": {meal.Quality.WholeGrainGEquiv, maxNumeric7_1}, "fruit_veg_g": {meal.Quality.FruitVegG, maxNumeric7_1},
	} {
		if value.value != nil && !validFiniteNumeric(*value.value, value.upperBound) {
			return apperr.Validation("fixed meal " + name + " must be finite and fit the database column")
		}
	}
	return nil
}

func validateNumeric6_1(name string, value float64) error {
	if !validFiniteNumeric(value, maxNumeric6_1) {
		return apperr.Validation(name + " must be finite and fit the database column")
	}
	return nil
}

func validFiniteNumeric(value, upperBound float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= upperBound
}

func validKcal(value int) bool {
	return value >= 0 && int64(value) <= math.MaxInt32
}

func optionalBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *Service) validateExclusion(ctx context.Context, exclusion eaterdom.Exclusion) error {
	if exclusion.TargetValue == "" {
		return apperr.Validation("exclusion target value must not be blank")
	}
	if exclusion.Severity != eaterdom.ExclusionSeverityHard && exclusion.Severity != eaterdom.ExclusionSeveritySoft {
		return apperr.Validation("exclusion severity is invalid")
	}
	switch exclusion.TargetKind {
	case eaterdom.ExclusionTargetFoodGroup:
		if !validFoodGroup(catalogdom.FoodGroup(exclusion.TargetValue)) {
			return apperr.Validation("exclusion food group is invalid")
		}
	case eaterdom.ExclusionTargetRoleTag:
		exists, err := s.eaters.RoleTagExists(ctx, exclusion.TargetValue)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.NotFound("role tag")
		}
	case eaterdom.ExclusionTargetProduct:
		productID, err := strconv.ParseInt(exclusion.TargetValue, 10, 64)
		if err != nil || productID <= 0 {
			return apperr.Validation("exclusion product id must be positive")
		}
		exists, err := s.eaters.ProductExists(ctx, productID)
		if err != nil {
			return err
		}
		if !exists {
			return apperr.NotFound("product")
		}
	default:
		return apperr.Validation("exclusion target kind is invalid")
	}
	return nil
}

func normalizeAllergens(values []eaterdom.Allergen) ([]eaterdom.Allergen, error) {
	result := make([]eaterdom.Allergen, 0, len(values))
	seen := make(map[eaterdom.Allergen]struct{}, len(values))
	for _, value := range values {
		value = eaterdom.Allergen(strings.TrimSpace(string(value)))
		if value == "" {
			return nil, apperr.Validation("allergen must not be blank")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func validSex(value eaterdom.Sex) bool {
	return value == eaterdom.SexMale || value == eaterdom.SexFemale || value == eaterdom.SexUnspecified
}

func validMealType(value eaterdom.MealType) bool {
	return value == eaterdom.MealTypeBreakfast || value == eaterdom.MealTypeLunch || value == eaterdom.MealTypeDinner || value == eaterdom.MealTypeSnack
}

func validFoodGroup(value catalogdom.FoodGroup) bool {
	switch value {
	case catalogdom.FoodGroupFruit, catalogdom.FoodGroupVegetable, catalogdom.FoodGroupPotato, catalogdom.FoodGroupGrain, catalogdom.FoodGroupLegume, catalogdom.FoodGroupNutsSeeds, catalogdom.FoodGroupDairy, catalogdom.FoodGroupDairyAlt, catalogdom.FoodGroupFishSeafood, catalogdom.FoodGroupEgg, catalogdom.FoodGroupRedMeat, catalogdom.FoodGroupWhiteMeat, catalogdom.FoodGroupProcessedMeat, catalogdom.FoodGroupMixed, catalogdom.FoodGroupSauceCondiment, catalogdom.FoodGroupConfectionery, catalogdom.FoodGroupBeverage, catalogdom.FoodGroupOther:
		return true
	default:
		return false
	}
}
