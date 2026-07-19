package eater

import "testing"

func TestSexValid(t *testing.T) {
	for _, value := range []Sex{SexMale, SexFemale, SexUnspecified} {
		if !value.Valid() {
			t.Errorf("Sex(%q).Valid() = false, want true", value)
		}
	}
	if Sex("invalid").Valid() {
		t.Error("Sex(\"invalid\").Valid() = true, want false")
	}
}

func TestMealTypeValid(t *testing.T) {
	for _, value := range []MealType{
		MealTypeBreakfast,
		MealTypeLunch,
		MealTypeDinner,
		MealTypeSnack,
	} {
		if !value.Valid() {
			t.Errorf("MealType(%q).Valid() = false, want true", value)
		}
	}
	if MealType("invalid").Valid() {
		t.Error("MealType(\"invalid\").Valid() = true, want false")
	}
}

func TestIDsValid(t *testing.T) {
	for value, want := range map[ID]bool{1: true, 0: false, -1: false} {
		if got := value.Valid(); got != want {
			t.Errorf("ID(%d).Valid() = %t, want %t", value, got, want)
		}
	}
	for value, want := range map[ExclusionID]bool{1: true, 0: false, -1: false} {
		if got := value.Valid(); got != want {
			t.Errorf("ExclusionID(%d).Valid() = %t, want %t", value, got, want)
		}
	}
}
