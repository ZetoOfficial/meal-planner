package catalog

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
	FoodGroup            *FoodGroup
	ProcessingLevel      *ProcessingLevel
	ProcessingConfidence *float64
	CountsToFruitVeg     *bool
	IsLegume             *bool
	IsWholeGrain         *bool
	WholeGrainRatio      *float64
	IsRedMeat            *bool
	IsProcessedMeat      *bool
	ClassificationNote   *string
}
