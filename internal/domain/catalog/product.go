package catalog

type ProductID int64

type Product struct {
	ID             ProductID
	Brand          string
	Name           string
	ProteinPer100G float64
	FatPer100G     float64
	CarbPer100G    float64
	KcalPer100G    float64
	PricePer100G   *float64
	DefaultUnit    string
	Purchasable    bool
	Notes          *string
	Nutrition      NutritionFacts
	DataQuality    NutritionData
	Classification FoodClassification
	Allergens      []Allergen
	Roles          []RoleTag
}

type FoodGroup string

const (
	FoodGroupFruit          FoodGroup = "fruit"
	FoodGroupVegetable      FoodGroup = "vegetable"
	FoodGroupPotato         FoodGroup = "potato"
	FoodGroupGrain          FoodGroup = "grain"
	FoodGroupLegume         FoodGroup = "legume"
	FoodGroupNutsSeeds      FoodGroup = "nuts_seeds"
	FoodGroupDairy          FoodGroup = "dairy"
	FoodGroupDairyAlt       FoodGroup = "dairy_alt"
	FoodGroupFishSeafood    FoodGroup = "fish_seafood"
	FoodGroupEgg            FoodGroup = "egg"
	FoodGroupRedMeat        FoodGroup = "red_meat"
	FoodGroupWhiteMeat      FoodGroup = "white_meat"
	FoodGroupProcessedMeat  FoodGroup = "processed_meat"
	FoodGroupMixed          FoodGroup = "mixed"
	FoodGroupSauceCondiment FoodGroup = "sauce_condiment"
	FoodGroupConfectionery  FoodGroup = "confectionery"
	FoodGroupBeverage       FoodGroup = "beverage"
	FoodGroupOther          FoodGroup = "other"
)

type ProcessingLevel string

const (
	ProcessingLevelMinimallyProcessed ProcessingLevel = "minimally_processed"
	ProcessingLevelProcessedCulinary  ProcessingLevel = "processed_culinary"
	ProcessingLevelProcessedFood      ProcessingLevel = "processed_food"
	ProcessingLevelUltraProcessed     ProcessingLevel = "ultra_processed"
	ProcessingLevelUnknown            ProcessingLevel = "unknown"
)

type FoodClassification struct {
	FoodGroup            FoodGroup
	ProcessingLevel      ProcessingLevel
	ProcessingConfidence *float64
	CountsToFruitVeg     bool
	IsLegume             bool
	IsWholeGrain         bool
	WholeGrainRatio      *float64
	IsRedMeat            bool
	IsProcessedMeat      bool
	Source               *string
	Note                 *string
}
