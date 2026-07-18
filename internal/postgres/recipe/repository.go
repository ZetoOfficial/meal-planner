package recipe

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

func (r *Repository) ResolveRoleTags(ctx context.Context, codes []string) (map[string]catalog.RoleTag, error) {
	rows, err := r.store.Queries(ctx).ResolveRecipeRoleTags(ctx, codes)
	if err != nil {
		return nil, dberror.Map(err, "role tag")
	}
	result := make(map[string]catalog.RoleTag, len(rows))
	for _, row := range rows {
		role := roleTagFromRow(row)
		result[role.Code] = role
	}
	return result, nil
}

func (r *Repository) Create(ctx context.Context, template recipedom.Template, roles map[string]catalog.RoleTag) (recipedom.Template, error) {
	query := r.store.Queries(ctx)
	stored, err := query.CreateRecipeTemplate(ctx, templateParams(template))
	if err != nil {
		return recipedom.Template{}, dberror.Map(err, "recipe")
	}
	for _, slot := range template.Slots {
		role, ok := roles[slot.RoleTagCode]
		if !ok {
			return recipedom.Template{}, apperr.Validation("recipe slot role does not exist", slot.RoleTagCode)
		}
		if _, err := query.CreateRecipeSlot(ctx, slotParams(stored.ID, slot, role)); err != nil {
			return recipedom.Template{}, dberror.Map(err, "recipe slot role")
		}
	}
	return r.load(ctx, query, recipedom.ID(stored.ID))
}

func (r *Repository) List(ctx context.Context, status *recipedom.Status) ([]recipedom.Template, error) {
	query := r.store.Queries(ctx)
	var (
		rows []sqlc.RecipeTemplate
		err  error
	)
	if status == nil {
		rows, err = query.ListRecipeTemplates(ctx)
	} else {
		rows, err = query.ListRecipeTemplatesByStatus(ctx, sqlc.RecipeStatus(*status))
	}
	if err != nil {
		return nil, dberror.Map(err, "recipe")
	}
	templates := make([]recipedom.Template, 0, len(rows))
	for i := range rows {
		template, err := r.templateWithSlots(ctx, query, rows[i])
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (r *Repository) Get(ctx context.Context, id recipedom.ID) (recipedom.Template, error) {
	return r.load(ctx, r.store.Queries(ctx), id)
}

func (r *Repository) Promote(ctx context.Context, id recipedom.ID, promotedAt time.Time) (recipedom.Template, error) {
	query := r.store.Queries(ctx)
	row, err := query.PromoteRecipeTemplate(ctx, sqlc.PromoteRecipeTemplateParams{
		ID:         int64(id),
		PromotedAt: promotedAtValue(promotedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return recipedom.Template{}, apperr.Conflict("only draft recipes can be promoted")
	}
	if err != nil {
		return recipedom.Template{}, dberror.Map(err, "recipe")
	}
	return r.templateWithSlots(ctx, query, row)
}

func (r *Repository) ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error) {
	status := recipedom.StatusPermanent
	return r.List(ctx, &status)
}

func (r *Repository) load(ctx context.Context, query *sqlc.Queries, id recipedom.ID) (recipedom.Template, error) {
	row, err := query.GetRecipeTemplate(ctx, int64(id))
	if err != nil {
		return recipedom.Template{}, dberror.Map(err, "recipe")
	}
	return r.templateWithSlots(ctx, query, row)
}

func (r *Repository) templateWithSlots(ctx context.Context, query *sqlc.Queries, row sqlc.RecipeTemplate) (recipedom.Template, error) {
	template, err := templateFromRow(row)
	if err != nil {
		return recipedom.Template{}, err
	}
	rows, err := query.ListRecipeSlotsByTemplate(ctx, row.ID)
	if err != nil {
		return recipedom.Template{}, dberror.Map(err, "recipe slots")
	}
	template.Slots = make([]recipedom.Slot, 0, len(rows))
	for _, row := range rows {
		slot, err := slotFromRow(row)
		if err != nil {
			return recipedom.Template{}, err
		}
		template.Slots = append(template.Slots, slot)
	}
	return template, nil
}
