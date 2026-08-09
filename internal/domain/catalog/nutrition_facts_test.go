package catalog

import "testing"

func TestNutritionFactsEffectiveSodium(t *testing.T) {
	directSodium := 320.0
	salt := 2.54

	for name, test := range map[string]struct {
		facts NutritionFacts
		want  *float64
	}{
		"direct": {
			facts: NutritionFacts{SodiumMgPer100G: &directSodium},
			want:  &directSodium,
		},
		"converted salt": {
			facts: NutritionFacts{SaltGPer100G: &salt},
			want:  float64Pointer(1000),
		},
		"direct wins": {
			facts: NutritionFacts{SodiumMgPer100G: &directSodium, SaltGPer100G: &salt},
			want:  &directSodium,
		},
		"unknown": {facts: NutritionFacts{}},
	} {
		t.Run(name, func(t *testing.T) {
			got := test.facts.EffectiveSodiumMgPer100G()
			if got == nil && test.want == nil {
				return
			}
			if got == nil || test.want == nil || *got != *test.want {
				t.Fatalf("EffectiveSodiumMgPer100G() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNutritionFactsCoverageCountsEffectiveSodiumOnce(t *testing.T) {
	sodium := 320.0
	salt := 2.54
	for name, facts := range map[string]NutritionFacts{
		"sodium":            {SodiumMgPer100G: &sodium},
		"salt":              {SaltGPer100G: &salt},
		"both alternatives": {SodiumMgPer100G: &sodium, SaltGPer100G: &salt},
	} {
		t.Run(name, func(t *testing.T) {
			if got, want := facts.Coverage(), 1.0/nutritionFactCoverageFields; got != want {
				t.Fatalf("Coverage() = %v, want %v", got, want)
			}
		})
	}
}

func TestNutritionFactsCoverageDistinguishesKnownZeroFromUnknown(t *testing.T) {
	zero := 0.0
	if got, want := (NutritionFacts{FiberPer100G: &zero}).Coverage(), 1.0/nutritionFactCoverageFields; got != want {
		t.Fatalf("known zero coverage = %v, want %v", got, want)
	}
	if got := (NutritionFacts{}).Coverage(); got != 0 {
		t.Fatalf("unknown coverage = %v, want 0", got)
	}
}

func TestProcessingLevelKnown(t *testing.T) {
	if ProcessingLevelUnknown.Known() || ProcessingLevel("").Known() {
		t.Fatal("unknown processing level reported as known")
	}
	if !ProcessingLevelMinimallyProcessed.Known() {
		t.Fatal("valid processing level reported as unknown")
	}
}

func float64Pointer(value float64) *float64 { return &value }
