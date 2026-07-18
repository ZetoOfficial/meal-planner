package recipe

import (
	"context"
	"strings"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
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
	RoleTagCode string
	Label       string
	Qty         float64
	Unit        string
	Position    int
	Optional    bool
}

type CreateRecipeCommand struct {
	Name          string
	Cuisine       *string
	CookingMethod *string
	Storage       recipedom.StorageProfile
	StorageNotes  *string
	Equipment     []string
	Instructions  *string
	Slots         []SlotCommand
}

func (s *Service) CreateRecipe(ctx context.Context, command CreateRecipeCommand) (recipedom.Template, error) {
	template := templateFromCommand(command)
	var created recipedom.Template
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		roles, err := s.recipes.ResolveRoleTags(ctx, roleCodes(template))
		if err != nil {
			return err
		}
		if err := validateTemplate(template, roles); err != nil {
			return err
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
	if status != nil && !validStatus(*status) {
		return nil, apperr.Validation("recipe status is invalid")
	}
	return s.recipes.List(ctx, status)
}

func (s *Service) GetRecipe(ctx context.Context, id recipedom.ID) (recipedom.Template, error) {
	if id <= 0 {
		return recipedom.Template{}, apperr.Validation("recipe id must be positive")
	}
	return s.recipes.Get(ctx, id)
}

func (s *Service) PromoteRecipe(ctx context.Context, id recipedom.ID) (recipedom.Template, error) {
	if id <= 0 {
		return recipedom.Template{}, apperr.Validation("recipe id must be positive")
	}
	var promoted recipedom.Template
	err := s.transactions.WithTx(ctx, func(ctx context.Context) error {
		template, err := s.recipes.Get(ctx, id)
		if err != nil {
			return err
		}
		roles, err := s.recipes.ResolveRoleTags(ctx, roleCodes(template))
		if err != nil {
			return err
		}
		if err := validateTemplate(template, roles); err != nil {
			return err
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

func templateFromCommand(command CreateRecipeCommand) recipedom.Template {
	template := recipedom.Template{
		Name:          strings.TrimSpace(command.Name),
		Status:        recipedom.StatusDraft,
		Source:        recipedom.SourceManual,
		Cuisine:       normalizedOptional(command.Cuisine),
		CookingMethod: normalizedOptional(command.CookingMethod),
		Storage:       command.Storage,
		StorageNotes:  normalizedOptional(command.StorageNotes),
		Equipment:     normalizedEquipment(command.Equipment),
		Instructions:  normalizedOptional(command.Instructions),
		Weight:        1,
		Slots:         make([]recipedom.Slot, len(command.Slots)),
	}
	for i, slot := range command.Slots {
		unit := strings.TrimSpace(slot.Unit)
		if unit == "" {
			unit = "g"
		}
		template.Slots[i] = recipedom.Slot{
			RoleTagCode: strings.TrimSpace(slot.RoleTagCode),
			Label:       strings.TrimSpace(slot.Label),
			Qty:         slot.Qty,
			Unit:        unit,
			Position:    slot.Position,
			Optional:    slot.Optional,
		}
	}
	return template
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

func roleCodes(template recipedom.Template) []string {
	result := make([]string, 0, len(template.Slots))
	seen := make(map[string]struct{}, len(template.Slots))
	for _, slot := range template.Slots {
		if _, exists := seen[slot.RoleTagCode]; exists {
			continue
		}
		seen[slot.RoleTagCode] = struct{}{}
		result = append(result, slot.RoleTagCode)
	}
	return result
}

func validateTemplate(template recipedom.Template, roles map[string]catalog.RoleTag) error {
	if err := recipedom.Validate(template, roles); err != nil {
		return apperr.Validation(err.Error())
	}
	return nil
}

func validStatus(status recipedom.Status) bool {
	switch status {
	case recipedom.StatusDraft, recipedom.StatusPermanent, recipedom.StatusRejected:
		return true
	default:
		return false
	}
}
