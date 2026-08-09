package httpapi

import (
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/api/openapi"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

func TestClassificationMappingPreservesUnknownAndKnownFalse(t *testing.T) {
	unknown := classificationCommand(nil)
	if unknown.FoodGroup != nil || unknown.IsLegume != nil {
		t.Fatalf("unknown classification = %#v, want nil fields", unknown)
	}

	knownFalse := false
	foodGroup := openapi.Other
	source := "manual"
	known := classificationCommand(&openapi.FoodClassification{
		FoodGroup:            &foodGroup,
		IsLegume:             &knownFalse,
		ClassificationSource: &source,
	})
	if known.FoodGroup == nil || *known.FoodGroup != catalogdom.FoodGroupOther {
		t.Fatalf("known food group = %v, want other", known.FoodGroup)
	}
	if known.IsLegume == nil || *known.IsLegume {
		t.Fatalf("known legume flag = %v, want false", known.IsLegume)
	}
	if known.Source == nil || *known.Source != source {
		t.Fatalf("classification source = %v, want %q", known.Source, source)
	}

	response := classificationResponse(known)
	if response.IsLegume == nil || *response.IsLegume || response.ClassificationSource == nil || *response.ClassificationSource != source {
		t.Fatalf("classification response = %#v, want known false and source", response)
	}
}
