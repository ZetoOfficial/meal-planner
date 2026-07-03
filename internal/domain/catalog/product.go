package catalog

type ProductID int64

type Product struct {
	ID             ProductID
	Brand          string
	Name           string
	ProteinPer100g float64
	FatPer100g     float64
	CarbPer100g    float64
	KcalPer100g    float64
	PricePer100g   *float64
	DefaultUnit    string
	Purchasable    bool
	Allergens      []string
	Roles          []string
	Nutrition      *NutritionFacts
	DataQuality    *NutritionDataQualityMetadata
	Classification *FoodClassification
}
