package recipe

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func TestRecipeMappingPreservesBatchContract(t *testing.T) {
	expectedOutput := 720.0
	preparedFactor := 0.75
	template := recipedom.Template{
		Name:             "Roasted chicken",
		Status:           recipedom.StatusDraft,
		Source:           recipedom.SourceManual,
		ComponentKind:    recipedom.ComponentKindProtein,
		AllowedMealTypes: []meal.Type{meal.TypeLunch, meal.TypeDinner},
		BaseServings:     4,
		ExpectedOutputG:  &expectedOutput,
		Storage:          recipedom.StorageKeeps3dReheat,
		Weight:           1,
		Slots: []recipedom.Slot{{
			RoleTagCode:       "protein:lean",
			Label:             "Protein",
			QtyG:              960,
			PreparedQtyFactor: &preparedFactor,
			Position:          0,
		}},
	}

	templateRow := templateParams(template)
	if templateRow.ComponentKind != sqlc.ComponentKindProtein ||
		templateRow.BaseServings != 4 ||
		!slices.Equal(templateRow.AllowedMealTypes, []string{"lunch", "dinner"}) {
		t.Fatalf("templateParams() = %#v", templateRow)
	}
	assertNumeric(t, templateRow.ExpectedOutputG, expectedOutput)

	slotRow := slotParams(7, template.Slots[0], catalog.RoleTag{ID: 9})
	assertNumeric(t, slotRow.QtyG, 960)
	assertNumeric(t, slotRow.PreparedQtyFactor, preparedFactor)

	loadedTemplate, err := templateFromRow(sqlc.RecipeTemplate{
		ID:               7,
		Name:             template.Name,
		Status:           sqlc.RecipeStatusDraft,
		Source:           sqlc.RecipeSourceManual,
		ComponentKind:    sqlc.ComponentKindProtein,
		AllowedMealTypes: []string{"lunch", "dinner"},
		BaseServings:     4,
		ExpectedOutputG:  pgvalue.Numeric(expectedOutput),
		Storage:          sqlc.StorageProfileKeeps3dReheat,
		Equipment:        []string{},
		Weight:           pgvalue.Numeric(1),
	})
	if err != nil {
		t.Fatalf("templateFromRow() error = %v", err)
	}
	if loadedTemplate.ComponentKind != template.ComponentKind ||
		loadedTemplate.BaseServings != template.BaseServings ||
		!slices.Equal(loadedTemplate.AllowedMealTypes, template.AllowedMealTypes) ||
		loadedTemplate.ExpectedOutputG == nil ||
		*loadedTemplate.ExpectedOutputG != expectedOutput {
		t.Fatalf("templateFromRow() = %#v", loadedTemplate)
	}

	loadedSlot, err := slotFromRow(sqlc.ListRecipeSlotsByTemplateRow{
		ID:                11,
		RoleCode:          "protein:lean",
		Label:             "Protein",
		QtyG:              pgvalue.Numeric(960),
		PreparedQtyFactor: pgvalue.Numeric(preparedFactor),
	})
	if err != nil {
		t.Fatalf("slotFromRow() error = %v", err)
	}
	if loadedSlot.QtyG != 960 ||
		loadedSlot.PreparedQtyFactor == nil ||
		*loadedSlot.PreparedQtyFactor != preparedFactor {
		t.Fatalf("slotFromRow() = %#v", loadedSlot)
	}
}

func assertNumeric(t *testing.T, value pgtype.Numeric, want float64) {
	t.Helper()
	got, err := pgvalue.Float64(value)
	if err != nil {
		t.Fatalf("convert numeric: %v", err)
	}
	if got != want {
		t.Fatalf("numeric = %v, want %v", got, want)
	}
}
