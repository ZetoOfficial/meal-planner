package feedback

import "testing"

func TestValidateRejectsUnknownTargetsAndRatings(t *testing.T) {
	high := 3
	cases := map[string]Event{
		"no eater":       {TargetType: TargetRecipeTemplate, TargetID: 1},
		"unknown target": {EaterID: 1, TargetType: "sandwich", TargetID: 1},
		"no target id":   {EaterID: 1, TargetType: TargetProduct},
		"rating too high": {
			EaterID: 1, TargetType: TargetProduct, TargetID: 1, Rating: &high,
		},
	}
	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			if err := event.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", name)
			}
		})
	}
}

func TestValidateAcceptsACommentWithoutARating(t *testing.T) {
	comment := "too salty"
	event := Event{EaterID: 1, TargetType: TargetRecipeTemplate, TargetID: 5, Comment: &comment}

	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a comment-only event to be valid", err)
	}
}

func TestWeightDeltaScalesWithTheRating(t *testing.T) {
	for rating, want := range map[int]float64{-2: -0.2, -1: -0.1, 1: 0.1, 2: 0.2} {
		value := rating
		delta := WeightDeltaFor(&value)
		if delta == nil || *delta != want {
			t.Fatalf("WeightDeltaFor(%d) = %v, want %v", rating, delta, want)
		}
	}
}

func TestWeightDeltaIsAbsentWithoutARating(t *testing.T) {
	zero := 0
	if delta := WeightDeltaFor(nil); delta != nil {
		t.Fatalf("WeightDeltaFor(nil) = %v, want no weight change", delta)
	}
	if delta := WeightDeltaFor(&zero); delta != nil {
		t.Fatalf("WeightDeltaFor(0) = %v, want no weight change for a neutral rating", delta)
	}
}

func TestWeightBoundsKeepTargetsInRotation(t *testing.T) {
	if MinWeight <= 0 {
		t.Fatalf("MinWeight = %v, want feedback to lower the odds rather than ban", MinWeight)
	}
	if MaxWeight <= MinWeight {
		t.Fatalf("weight bounds = [%v, %v], want a usable range", MinWeight, MaxWeight)
	}
}
