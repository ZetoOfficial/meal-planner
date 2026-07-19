package recipe

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/recipe/mocks"
)

func TestCreateRecipeDefaultsAndPersistsValidManualDraft(t *testing.T) {
	service, repository, transactions, _ := newMockService(t)
	expectPassthroughTx(transactions)
	roles := validRoles()
	repository.EXPECT().
		ResolveRoleTags(gomock.Any(), []string{"protein:lean"}).
		Return(roles, nil)

	var stored recipedom.Template
	repository.EXPECT().
		Create(gomock.Any(), gomock.Any(), roles).
		DoAndReturn(func(_ context.Context, template recipedom.Template, _ map[string]catalog.RoleTag) (recipedom.Template, error) {
			stored = template
			template.ID = 42
			return template, nil
		})

	created, err := service.CreateRecipe(t.Context(), CreateRecipeCommand{
		Name:      " Chicken bowl ",
		Storage:   recipedom.StorageKeeps3dReheat,
		Equipment: []string{" oven ", "oven"},
		Slots: []SlotCommand{{
			RoleTagCode: " protein:lean ",
			Label:       " Protein ",
			Qty:         180,
			Position:    0,
		}},
	})
	if err != nil {
		t.Fatalf("CreateRecipe() error = %v", err)
	}
	if created.ID != 42 || stored.Status != recipedom.StatusDraft || stored.Source != recipedom.SourceManual {
		t.Fatalf("created = %#v, stored = %#v, want id/manual draft", created, stored)
	}
	if stored.Name != "Chicken bowl" || stored.Weight != 1 || stored.CookedCount != 0 {
		t.Fatalf("stored template = %#v, want normalized defaults", stored)
	}
	if !slices.Equal(stored.Equipment, []string{"oven"}) || stored.Slots[0].Unit != "g" {
		t.Fatalf("stored equipment/slots = %#v/%#v, want normalized values", stored.Equipment, stored.Slots)
	}
}

func TestCreateRecipeCommandTemplateMapsNormalizedSlots(t *testing.T) {
	cuisine := " Italian "
	command := CreateRecipeCommand{
		Name:      " Chicken bowl ",
		Cuisine:   &cuisine,
		Storage:   recipedom.StorageKeeps3d,
		Equipment: []string{" oven ", "oven"},
		Slots: []SlotCommand{{
			RoleTagCode: " protein:lean ",
			Label:       " Protein ",
			Qty:         180,
		}},
	}

	template := command.template()

	if template.Name != "Chicken bowl" || template.Cuisine == nil || *template.Cuisine != "Italian" {
		t.Fatalf("CreateRecipeCommand.template() = %#v, want normalized metadata", template)
	}
	if template.Status != recipedom.StatusDraft || template.Source != recipedom.SourceManual || template.Weight != 1 {
		t.Fatalf("CreateRecipeCommand.template() = %#v, want manual draft defaults", template)
	}
	if !slices.Equal(template.Equipment, []string{"oven"}) || len(template.Slots) != 1 {
		t.Fatalf("CreateRecipeCommand.template() equipment/slots = %#v/%#v", template.Equipment, template.Slots)
	}
	if template.Slots[0].RoleTagCode != "protein:lean" || template.Slots[0].Label != "Protein" || template.Slots[0].Unit != "g" {
		t.Fatalf("SlotCommand.slot() = %#v, want normalized slot defaults", template.Slots[0])
	}
}

func TestCreateRecipeRejectsMissingRole(t *testing.T) {
	service, repository, transactions, _ := newMockService(t)
	expectPassthroughTx(transactions)
	repository.EXPECT().
		ResolveRoleTags(gomock.Any(), []string{"protein:lean"}).
		Return(map[string]catalog.RoleTag{}, nil)

	_, err := service.CreateRecipe(t.Context(), CreateRecipeCommand{
		Name:    "Chicken bowl",
		Storage: recipedom.StorageKeeps3d,
		Slots: []SlotCommand{{
			RoleTagCode: "protein:lean",
			Label:       "Protein",
			Qty:         180,
			Position:    0,
		}},
	})
	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestListRecipesRejectsInvalidStatus(t *testing.T) {
	service, _, _, _ := newMockService(t)
	status := recipedom.Status("unknown")

	_, err := service.ListRecipes(t.Context(), &status)

	assertErrorCode(t, err, apperr.CodeValidation)
}

func TestPromoteRecipeAllowsValidLLMDraft(t *testing.T) {
	service, repository, transactions, clock := newMockService(t)
	expectPassthroughTx(transactions)
	draft := validTemplate()
	draft.ID = 7
	draft.Source = recipedom.SourceLLM
	roles := validRoles()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)

	repository.EXPECT().Get(gomock.Any(), recipedom.ID(7)).Return(draft, nil)
	repository.EXPECT().
		ResolveRoleTags(gomock.Any(), []string{"protein:lean"}).
		Return(roles, nil)
	clock.EXPECT().Now().Return(now)
	repository.EXPECT().
		Promote(gomock.Any(), recipedom.ID(7), now).
		DoAndReturn(func(_ context.Context, _ recipedom.ID, promotedAt time.Time) (recipedom.Template, error) {
			result := draft
			result.Status = recipedom.StatusPermanent
			result.PromotedAt = &promotedAt
			result.CookedCount++
			return result, nil
		})

	promoted, err := service.PromoteRecipe(t.Context(), 7)
	if err != nil {
		t.Fatalf("PromoteRecipe() error = %v", err)
	}
	if promoted.Status != recipedom.StatusPermanent || promoted.CookedCount != 1 || promoted.PromotedAt == nil || !promoted.PromotedAt.Equal(now) {
		t.Fatalf("PromoteRecipe() = %#v, want promoted recipe", promoted)
	}
}

func TestPromoteRecipeRejectsNonDraftTemplate(t *testing.T) {
	for _, status := range []recipedom.Status{recipedom.StatusPermanent, recipedom.StatusRejected} {
		t.Run(string(status), func(t *testing.T) {
			service, repository, transactions, _ := newMockService(t)
			expectPassthroughTx(transactions)
			template := validTemplate()
			template.ID = 7
			template.Status = status
			repository.EXPECT().Get(gomock.Any(), recipedom.ID(7)).Return(template, nil)
			repository.EXPECT().
				ResolveRoleTags(gomock.Any(), []string{"protein:lean"}).
				Return(validRoles(), nil)

			_, err := service.PromoteRecipe(t.Context(), 7)

			assertErrorCode(t, err, apperr.CodeConflict)
		})
	}
}

func TestPromoteRecipeReturnsNotFound(t *testing.T) {
	service, repository, transactions, _ := newMockService(t)
	expectPassthroughTx(transactions)
	repository.EXPECT().
		Get(gomock.Any(), recipedom.ID(404)).
		Return(recipedom.Template{}, apperr.NotFound("recipe"))

	_, err := service.PromoteRecipe(t.Context(), 404)

	assertErrorCode(t, err, apperr.CodeNotFound)
}

func TestListPermanentCandidatesReturnsRepositoryResults(t *testing.T) {
	service, repository, _, _ := newMockService(t)
	permanent := validTemplate()
	permanent.Status = recipedom.StatusPermanent
	repository.EXPECT().
		ListPermanentCandidates(gomock.Any()).
		Return([]recipedom.Template{permanent}, nil)

	candidates, err := service.ListPermanentCandidates(t.Context())
	if err != nil {
		t.Fatalf("ListPermanentCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Status != recipedom.StatusPermanent {
		t.Fatalf("ListPermanentCandidates() = %#v", candidates)
	}
}

func newMockService(t *testing.T) (*Service, *mocks.MockRecipeRepository, *mocks.MockTxRunner, *mocks.MockClock) {
	t.Helper()
	controller := gomock.NewController(t)
	repository := mocks.NewMockRecipeRepository(controller)
	transactions := mocks.NewMockTxRunner(controller)
	clock := mocks.NewMockClock(controller)
	return NewService(repository, transactions, clock), repository, transactions, clock
}

func expectPassthroughTx(transactions *mocks.MockTxRunner) {
	transactions.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func validTemplate() recipedom.Template {
	return recipedom.Template{
		Name:        "Chicken bowl",
		Status:      recipedom.StatusDraft,
		Source:      recipedom.SourceManual,
		Storage:     recipedom.StorageKeeps3d,
		Weight:      1,
		CookedCount: 0,
		Slots: []recipedom.Slot{{
			RoleTagCode: "protein:lean",
			Label:       "Protein",
			Qty:         180,
			Unit:        "g",
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

func assertErrorCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	var appError *apperr.Error
	if !errors.As(err, &appError) || appError.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}
