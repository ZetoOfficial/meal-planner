package catalog

import "time"

type NutritionFacts struct {
	SaturatedFatPer100g               *float64
	TransFatPer100g                   *float64
	IndustrialTransFatPer100g         *float64
	FiberPer100g                      *float64
	SodiumMgPer100g                   *float64
	SaltGPer100g                      *float64
	TotalSugarsPer100g                *float64
	AddedSugarsPer100g                *float64
	FreeSugarsPer100g                 *float64
	ContainsPartiallyHydrogenatedOils *bool
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

type NutritionDataQualityMetadata struct {
	NutritionDataQuality   *NutritionDataQuality
	NutritionCoverageScore *float64
	SourceKind             *SourceKind
	SourceNote             *string
	LastVerifiedAt         *time.Time
}
