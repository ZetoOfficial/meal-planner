package recipe

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
)

func TestBatchComponentContractShape(t *testing.T) {
	templateType := reflect.TypeOf(Template{})
	for _, name := range []string{"ComponentKind", "AllowedMealTypes", "BaseServings", "ExpectedOutputG"} {
		if _, ok := templateType.FieldByName(name); !ok {
			t.Errorf("Template is missing %s", name)
		}
	}

	slotType := reflect.TypeOf(Slot{})
	for _, name := range []string{"QtyG", "PreparedQtyFactor"} {
		if _, ok := slotType.FieldByName(name); !ok {
			t.Errorf("Slot is missing %s", name)
		}
	}
	for _, name := range []string{"Qty", "Unit"} {
		if _, ok := slotType.FieldByName(name); ok {
			t.Errorf("Slot still exposes legacy %s", name)
		}
	}
}

func TestValidateAcceptsRoleBasedTemplate(t *testing.T) {
	template := validTemplate()
	roles := map[string]catalog.RoleTag{
		"protein:lean": {
			ID:    1,
			Code:  "protein:lean",
			Kind:  catalog.RoleKindProtein,
			Label: "Lean protein",
		},
	}

	if err := template.Validate(roles); err != nil {
		t.Fatalf("Template.Validate() error = %v", err)
	}
}

func TestValidateAcceptsRootRoleMatchingItsKind(t *testing.T) {
	template := validTemplate()
	template.Slots[0].RoleTagCode = "protein"
	roles := map[string]catalog.RoleTag{
		"protein": {
			ID:    1,
			Code:  "protein",
			Kind:  catalog.RoleKindProtein,
			Label: "Protein",
		},
	}

	if err := template.Validate(roles); err != nil {
		t.Fatalf("Template.Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidTemplate(t *testing.T) {
	tests := map[string]struct {
		mutate  func(*Template)
		roles   map[string]catalog.RoleTag
		message string
	}{
		"blank name": {
			mutate:  func(template *Template) { template.Name = " " },
			roles:   validRoles(),
			message: "name",
		},
		"unknown storage": {
			mutate:  func(template *Template) { template.Storage = "unknown" },
			roles:   validRoles(),
			message: "storage",
		},
		"unknown component kind": {
			mutate:  func(template *Template) { template.ComponentKind = "unknown" },
			roles:   validRoles(),
			message: "component kind",
		},
		"empty allowed meal types": {
			mutate:  func(template *Template) { template.AllowedMealTypes = nil },
			roles:   validRoles(),
			message: "meal type",
		},
		"duplicate allowed meal type": {
			mutate: func(template *Template) {
				template.AllowedMealTypes = []meal.Type{meal.TypeLunch, meal.TypeLunch}
			},
			roles:   validRoles(),
			message: "meal type",
		},
		"nonpositive base servings": {
			mutate:  func(template *Template) { template.BaseServings = 0 },
			roles:   validRoles(),
			message: "base servings",
		},
		"nonpositive expected output": {
			mutate: func(template *Template) {
				value := 0.0
				template.ExpectedOutputG = &value
			},
			roles:   validRoles(),
			message: "expected output",
		},
		"empty slots": {
			mutate:  func(template *Template) { template.Slots = nil },
			roles:   validRoles(),
			message: "slot",
		},
		"missing role": {
			mutate:  func(*Template) {},
			roles:   map[string]catalog.RoleTag{},
			message: "role",
		},
		"incompatible role kind": {
			mutate: func(*Template) {},
			roles: map[string]catalog.RoleTag{
				"protein:lean": {
					ID:    1,
					Code:  "protein:lean",
					Kind:  catalog.RoleKindVeg,
					Label: "Broken role",
				},
			},
			message: "kind",
		},
		"nonpositive quantity": {
			mutate:  func(template *Template) { template.Slots[0].QtyG = 0 },
			roles:   validRoles(),
			message: "quantity",
		},
		"quantity precision overflow": {
			mutate:  func(template *Template) { template.Slots[0].QtyG = 1_000_000 },
			roles:   validRoles(),
			message: "quantity",
		},
		"nonpositive prepared quantity factor": {
			mutate: func(template *Template) {
				value := 0.0
				template.Slots[0].PreparedQtyFactor = &value
			},
			roles:   validRoles(),
			message: "prepared quantity factor",
		},
		"position integer overflow": {
			mutate:  func(template *Template) { template.Slots[0].Position = 1 << 31 },
			roles:   validRoles(),
			message: "position",
		},
		"weight precision overflow": {
			mutate:  func(template *Template) { template.Weight = 100 },
			roles:   validRoles(),
			message: "weight",
		},
		"duplicate position": {
			mutate: func(template *Template) {
				template.Slots = append(template.Slots, Slot{
					RoleTagCode: "protein:lean",
					Label:       "Second protein",
					QtyG:        100,
					Position:    template.Slots[0].Position,
				})
			},
			roles:   validRoles(),
			message: "position",
		},
		"blank equipment": {
			mutate:  func(template *Template) { template.Equipment = []string{"oven", " "} },
			roles:   validRoles(),
			message: "equipment",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			template := validTemplate()
			test.mutate(&template)

			err := template.Validate(test.roles)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Template.Validate() error = %v, want containing %q", err, test.message)
			}
		})
	}
}

func TestTemplateRoleCodesPreservesFirstSlotOrder(t *testing.T) {
	template := validTemplate()
	template.Slots = append(template.Slots,
		Slot{RoleTagCode: "veg:green", Position: 1},
		Slot{RoleTagCode: "protein:lean", Position: 2},
	)

	got := template.RoleCodes()
	want := []string{"protein:lean", "veg:green"}
	if len(got) != len(want) {
		t.Fatalf("Template.RoleCodes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Template.RoleCodes() = %v, want %v", got, want)
		}
	}
}

func TestRecipeEnumValidity(t *testing.T) {
	for value, want := range map[Status]bool{
		StatusDraft:       true,
		StatusPermanent:   true,
		StatusRejected:    true,
		Status("unknown"): false,
	} {
		if got := value.Valid(); got != want {
			t.Errorf("Status(%q).Valid() = %t, want %t", value, got, want)
		}
	}
	for value, want := range map[Source]bool{
		SourceLLM:         true,
		SourceManual:      true,
		Source("unknown"): false,
	} {
		if got := value.Valid(); got != want {
			t.Errorf("Source(%q).Valid() = %t, want %t", value, got, want)
		}
	}
	for value, want := range map[ComponentKind]bool{
		ComponentKindProtein:          true,
		ComponentKindCarbSide:         true,
		ComponentKindVegetableSide:    true,
		ComponentKindSauce:            true,
		ComponentKindMixedMain:        true,
		ComponentKind("unknown_kind"): false,
	} {
		if got := value.Valid(); got != want {
			t.Errorf("ComponentKind(%q).Valid() = %t, want %t", value, got, want)
		}
	}
}

func TestIDValid(t *testing.T) {
	for value, want := range map[ID]bool{
		1:  true,
		0:  false,
		-1: false,
	} {
		if got := value.Valid(); got != want {
			t.Errorf("ID(%d).Valid() = %t, want %t", value, got, want)
		}
	}
}

func TestStorageProfileSupportsMultiDay(t *testing.T) {
	for profile, want := range map[StorageProfile]bool{
		StorageKeeps3d:       true,
		StorageKeeps3dReheat: true,
		StorageAssembleFresh: true,
		StorageEatSameDay:    false,
		StorageProfile(""):   false,
	} {
		if got := profile.SupportsMultiDay(); got != want {
			t.Errorf("%q.SupportsMultiDay() = %t, want %t", profile, got, want)
		}
	}
}

func validTemplate() Template {
	return Template{
		Name:          "Chicken bowl",
		Status:        StatusDraft,
		Source:        SourceManual,
		ComponentKind: ComponentKindProtein,
		AllowedMealTypes: []meal.Type{
			meal.TypeLunch,
			meal.TypeDinner,
		},
		BaseServings: 4,
		Storage:      StorageKeeps3dReheat,
		Equipment:    []string{"oven"},
		Weight:       1,
		CookedCount:  0,
		Slots: []Slot{{
			RoleTagCode: "protein:lean",
			Label:       "Protein",
			QtyG:        180,
			Position:    0,
		}},
	}
}

func validRoles() map[string]catalog.RoleTag {
	return map[string]catalog.RoleTag{
		"protein:lean": {
			ID:    1,
			Code:  "protein:lean",
			Kind:  catalog.RoleKindProtein,
			Label: "Lean protein",
		},
	}
}
