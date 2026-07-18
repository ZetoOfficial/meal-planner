package catalog

import "time"

const nutritionFactCoverageFields = 10

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
		facts.SodiumMgPer100G,
		facts.SaltGPer100G,
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

type NutritionDataQuality string

const (
	NutritionDataQualityLabelExact NutritionDataQuality = "label_exact"
	NutritionDataQualityTrustedDB  NutritionDataQuality = "trusted_db"
	NutritionDataQualityEstimated  NutritionDataQuality = "estimated"
	NutritionDataQualityUnknown    NutritionDataQuality = "unknown"
)

type SourceKind string

const (
	SourceKindPackLabel        SourceKind = "pack_label"
	SourceKindStoreCard        SourceKind = "store_card"
	SourceKindCompositionTable SourceKind = "composition_table"
	SourceKindManualEstimate   SourceKind = "manual_estimate"
)

type NutritionData struct {
	Quality        NutritionDataQuality
	Coverage       float64
	SourceKind     *SourceKind
	SourceNote     *string
	LastVerifiedAt *time.Time
}
