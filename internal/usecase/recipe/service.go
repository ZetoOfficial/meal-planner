package recipe

import (
	"context"
	"strings"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	"github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type Service struct {
	recipes      RecipeRepository
	transactions TxRunner
	clock        Clock
}

func NewService(recipes RecipeRepository, transactions TxRunner, clock Clock) *Service {
	if transactions == nil {
		panic("recipe transaction runner is required")
	}
	return &Service{recipes: recipes, transactions: transactions, clock: clock}
}

type SlotCommand struct {
	RoleTagCode       string
	Label             string
	QtyG              float64
	PreparedQtyFactor *float64
	Position          int
	Optional          bool
}

type CreateRecipeCommand struct {
	Name             string
	ComponentKind    recipedom.ComponentKind
	AllowedMealTypes []meal.Type
	BaseServings     int
	ExpectedOutputG  *float64
	Cuisine          *string
	CookingMethod    *string
	Storage          recipedom.StorageProfile
	StorageNotes     *string
	Equipment        []string
	Instructions     *string
	Slots            []SlotCommand
}

func (s *Service) CreateRecipe(ctx context.Context, command CreateRecipeCommand) (recipedom.Template, error) {
	template := command.template()
	var created recipedom.Template
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		roles, err := s.recipes.ResolveRoleTags(ctx, template.RoleCodes())
		if err != nil {
			return err
		}
		if err := template.Validate(roles); err != nil {
			return apperr.Validation(err.Error())
		}
		created, err = s.recipes.Create(ctx, template, roles)
		return err
	})
	if err != nil {
		return recipedom.Template{}, err
	}
	return created, nil
}

func (s *Service) ListRecipes(ctx context.Context, status *recipedom.Status) ([]recipedom.Template, error) {
	if status != nil && !status.Valid() {
		return nil, apperr.Validation("recipe status is invalid")
	}
	return s.recipes.List(ctx, status)
}

func (s *Service) GetRecipe(ctx context.Context, id recipedom.ID) (recipedom.Template, error) {
	if !id.Valid() {
		return recipedom.Template{}, apperr.Validation("recipe id must be positive")
	}
	return s.recipes.Get(ctx, id)
}

func (s *Service) PromoteRecipe(ctx context.Context, id recipedom.ID) (recipedom.Template, error) {
	if !id.Valid() {
		return recipedom.Template{}, apperr.Validation("recipe id must be positive")
	}
	var promoted recipedom.Template
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		template, err := s.recipes.Get(ctx, id)
		if err != nil {
			return err
		}
		roles, err := s.recipes.ResolveRoleTags(ctx, template.RoleCodes())
		if err != nil {
			return err
		}
		if err := template.Validate(roles); err != nil {
			return apperr.Validation(err.Error())
		}
		if template.Status != recipedom.StatusDraft {
			return apperr.Conflict("only draft recipes can be promoted")
		}
		promoted, err = s.recipes.Promote(ctx, id, s.clock.Now())
		return err
	})
	if err != nil {
		return recipedom.Template{}, err
	}
	return promoted, nil
}

func (s *Service) ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error) {
	return s.recipes.ListPermanentCandidates(ctx)
}

func (command CreateRecipeCommand) template() recipedom.Template {
	template := recipedom.Template{
		Name:             strings.TrimSpace(command.Name),
		Status:           recipedom.StatusDraft,
		Source:           recipedom.SourceManual,
		ComponentKind:    command.ComponentKind,
		AllowedMealTypes: append([]meal.Type(nil), command.AllowedMealTypes...),
		BaseServings:     command.BaseServings,
		ExpectedOutputG:  float64Pointer(command.ExpectedOutputG),
		Cuisine:          normalizedOptional(command.Cuisine),
		CookingMethod:    normalizedOptional(command.CookingMethod),
		Storage:          command.Storage,
		StorageNotes:     normalizedOptional(command.StorageNotes),
		Equipment:        normalizedEquipment(command.Equipment),
		Instructions:     normalizedOptional(command.Instructions),
		Weight:           1,
		Slots:            make([]recipedom.Slot, len(command.Slots)),
	}
	for i, slot := range command.Slots {
		template.Slots[i] = slot.slot()
	}
	return template
}

func (command SlotCommand) slot() recipedom.Slot {
	return recipedom.Slot{
		RoleTagCode:       strings.TrimSpace(command.RoleTagCode),
		Label:             strings.TrimSpace(command.Label),
		QtyG:              command.QtyG,
		PreparedQtyFactor: float64Pointer(command.PreparedQtyFactor),
		Position:          command.Position,
		Optional:          command.Optional,
	}
}

func float64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizedOptional(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func normalizedEquipment(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
