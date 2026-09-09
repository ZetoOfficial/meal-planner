package quality

import "github.com/ZetoOfficial/meal-planner/internal/domain/eater"

const (
	fiberMinFemaleG          = 25.0
	fiberMinMaleG            = 35.0
	sodiumMaxDailyMg         = 2300.0
	addedSugarMaxPctKcal     = 10.0
	addedSugarWarnPctKcal    = 5.0
	satFatMaxPctKcal         = 10.0
	fruitVegMinG             = 400.0
	wholeGrainRatioMin       = 0.5
	legumeServingSizeG       = 150.0
	redMeatMaxWeeklyG        = 350.0
	equalRuleWeight          = 0.10
	defaultCoverageThreshold = 0.5
)

const (
	RuleFiber              = "fiber_min_g"
	RuleSodium             = "sodium_max_daily_mg"
	RuleAddedSugar         = "added_sugar_max_pct_kcal"
	RuleSatFat             = "sat_fat_max_pct_kcal"
	RuleIndustrialTransFat = "industrial_trans_fat"
	RuleFruitVeg           = "fruit_veg_min_g"
	RuleWholeGrain         = "whole_grain_ratio"
	RuleLegumes            = "legumes_min"
	RuleRedMeat            = "red_meat_max_weekly_g"
	RuleProcessedMeat      = "processed_meat"
	RuleUPF                = "upf_share"
)

// AllRules lists every MVP rule code, in evaluation order.
func AllRules() []string {
	return []string{
		RuleFiber, RuleSodium, RuleAddedSugar, RuleSatFat, RuleIndustrialTransFat,
		RuleFruitVeg, RuleWholeGrain, RuleUPF, RuleLegumes, RuleRedMeat, RuleProcessedMeat,
	}
}

type RuleSeverity string

const (
	SeverityHard      RuleSeverity = "hard"
	SeveritySoft      RuleSeverity = "soft"
	SeverityWarn      RuleSeverity = "warn"
	SeverityAnalytics RuleSeverity = "analytics"
)

type Ruleset struct {
	FiberMinG map[eater.Sex]float64

	SodiumMaxDailyMg      float64
	AddedSugarMaxPctKcal  float64
	AddedSugarWarnPctKcal float64
	SatFatMaxPctKcal      float64
	FruitVegMinG          float64
	WholeGrainRatioMin    float64
	LegumeServingSizeG    float64
	RedMeatMaxWeeklyG     float64

	RuleWeights map[string]float64

	CoverageThreshold float64
}

func DefaultRuleset() Ruleset {
	return Ruleset{
		FiberMinG: map[eater.Sex]float64{
			eater.SexFemale:      fiberMinFemaleG,
			eater.SexMale:        fiberMinMaleG,
			eater.SexUnspecified: fiberMinFemaleG,
		},
		SodiumMaxDailyMg:      sodiumMaxDailyMg,
		AddedSugarMaxPctKcal:  addedSugarMaxPctKcal,
		AddedSugarWarnPctKcal: addedSugarWarnPctKcal,
		SatFatMaxPctKcal:      satFatMaxPctKcal,
		FruitVegMinG:          fruitVegMinG,
		WholeGrainRatioMin:    wholeGrainRatioMin,
		LegumeServingSizeG:    legumeServingSizeG,
		RedMeatMaxWeeklyG:     redMeatMaxWeeklyG,
		RuleWeights: map[string]float64{
			RuleFiber:         equalRuleWeight,
			RuleSodium:        equalRuleWeight,
			RuleAddedSugar:    equalRuleWeight,
			RuleSatFat:        equalRuleWeight,
			RuleFruitVeg:      equalRuleWeight,
			RuleWholeGrain:    equalRuleWeight,
			RuleLegumes:       equalRuleWeight,
			RuleRedMeat:       equalRuleWeight,
			RuleProcessedMeat: equalRuleWeight,
			RuleUPF:           equalRuleWeight,
		},
		CoverageThreshold: defaultCoverageThreshold,
	}
}
