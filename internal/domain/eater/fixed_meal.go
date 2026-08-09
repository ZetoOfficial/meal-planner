package eater

import (
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

type FixedMealID int64

type MealType = meal.Type

const (
	MealTypeBreakfast = meal.TypeBreakfast
	MealTypeLunch     = meal.TypeLunch
	MealTypeDinner    = meal.TypeDinner
	MealTypeSnack     = meal.TypeSnack
)

type Macros = nutrition.Macros

type FixedMealQuality struct {
	SaturatedFatG    *float64
	TransFatG        *float64
	FiberG           *float64
	SodiumMg         *float64
	AddedSugarsG     *float64
	FreeSugarsG      *float64
	ProcessedMeatG   *float64
	RedMeatG         *float64
	GrainGEquiv      *float64
	WholeGrainGEquiv *float64
	LegumeG          *float64
	FruitVegG        *float64
	UPFKcal          *float64

	IncludeInAnalytics bool
	IncludeInWarnings  bool
}

type FixedMeal struct {
	ID       FixedMealID
	EaterID  ID
	MealType MealType
	Label    string
	Macros   Macros
	Quality  FixedMealQuality
	Active   bool
}
