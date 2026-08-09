package catalog

import (
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

const nutritionFactCoverageFields = 9

type NutritionFacts struct {
	SaturatedFatPer100G               *float64
	TransFatPer100G                   *float64
	IndustrialTransFatPer100G         *float64
	FiberPer100G                      *float64
	SodiumMgPer100G                   *float64
	SaltGPer100G                      *float64
	TotalSugarsPer100G                *float64
	AddedSugarsPer100G                *float64
	FreeSugarsPer100G                 *float64
	ContainsPartiallyHydrogenatedOils *bool
}

func (facts NutritionFacts) Coverage() float64 {
	populated := 0
	for _, value := range []*float64{
		facts.SaturatedFatPer100G,
		facts.TransFatPer100G,
		facts.IndustrialTransFatPer100G,
		facts.FiberPer100G,
		facts.EffectiveSodiumMgPer100G(),
		facts.TotalSugarsPer100G,
		facts.AddedSugarsPer100G,
		facts.FreeSugarsPer100G,
	} {
		if value != nil {
			populated++
		}
	}
	if facts.ContainsPartiallyHydrogenatedOils != nil {
		populated++
	}
	return float64(populated) / nutritionFactCoverageFields
}

// EffectiveSodiumMgPer100G prefers explicit sodium and falls back to converted salt.
func (facts NutritionFacts) EffectiveSodiumMgPer100G() *float64 {
	if facts.SodiumMgPer100G != nil {
		return facts.SodiumMgPer100G
	}
	if facts.SaltGPer100G == nil {
		return nil
	}
	sodium := nutrition.SodiumMgFromSaltG(*facts.SaltGPer100G)
	return &sodium
}

type NutritionDataQuality string

const (
	NutritionDataQualityLabelExact NutritionDataQuality = "label_exact"
	NutritionDataQualityTrustedDB  NutritionDataQuality = "trusted_db"
	NutritionDataQualityEstimated  NutritionDataQuality = "estimated"
	NutritionDataQualityUnknown    NutritionDataQuality = "unknown"
)

func (q NutritionDataQuality) Valid() bool {
	switch q {
	case NutritionDataQualityLabelExact,
		NutritionDataQualityTrustedDB,
		NutritionDataQualityEstimated,
		NutritionDataQualityUnknown:
		return true
	default:
		return false
	}
}

type SourceKind string

const (
	SourceKindPackLabel        SourceKind = "pack_label"
	SourceKindStoreCard        SourceKind = "store_card"
	SourceKindCompositionTable SourceKind = "composition_table"
	SourceKindManualEstimate   SourceKind = "manual_estimate"
)

func (k SourceKind) Valid() bool {
	switch k {
	case SourceKindPackLabel,
		SourceKindStoreCard,
		SourceKindCompositionTable,
		SourceKindManualEstimate:
		return true
	default:
		return false
	}
}

type NutritionData struct {
	Quality        NutritionDataQuality
	Coverage       float64
	SourceKind     *SourceKind
	SourceNote     *string
	LastVerifiedAt *time.Time
}
