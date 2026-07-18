package catalog

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
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

func (r *Repository) Create(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	query := r.store.Queries(ctx)
	stored, err := query.CreateProduct(ctx, productParams(product))
	if err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product")
	}
	product.ID = catalogdom.ProductID(stored.ID)
	if err := r.writeAggregate(ctx, query, product); err != nil {
		return catalogdom.Product{}, err
	}
	return r.load(ctx, query, product.ID)
}

func (r *Repository) List(ctx context.Context, roleCode string) ([]catalogdom.Product, error) {
	query := r.store.Queries(ctx)
	var (
		rows []sqlc.Product
		err  error
	)
	if roleCode == "" {
		rows, err = query.ListProducts(ctx)
	} else {
		rows, err = query.ListProductsByRole(ctx, roleCode)
	}
	if err != nil {
		return nil, dberror.Map(err, "product")
	}
	products := make([]catalogdom.Product, 0, len(rows))
	for i := range rows {
		product, err := r.load(ctx, query, catalogdom.ProductID(rows[i].ID))
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *Repository) Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	return r.load(ctx, r.store.Queries(ctx), id)
}

func (r *Repository) Update(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error) {
	query := r.store.Queries(ctx)
	if _, err := query.UpdateProduct(ctx, updateProductParams(product)); err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product")
	}
	if err := r.writeAggregate(ctx, query, product); err != nil {
		return catalogdom.Product{}, err
	}
	return r.load(ctx, query, product.ID)
}

func (r *Repository) Delete(ctx context.Context, id catalogdom.ProductID) error {
	deleted, err := r.store.Queries(ctx).DeleteProduct(ctx, int64(id))
	if err != nil {
		return dberror.Map(err, "product")
	}
	if deleted == 0 {
		return apperr.NotFound("product")
	}
	return nil
}

func (r *Repository) GetRoleTag(ctx context.Context, code string) (catalogdom.RoleTag, error) {
	row, err := r.store.Queries(ctx).GetRoleTagByCode(ctx, code)
	if err != nil {
		return catalogdom.RoleTag{}, dberror.Map(err, "role tag")
	}
	return roleTagFromRow(row), nil
}

func (r *Repository) ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error) {
	rows, err := r.store.Queries(ctx).ListRoleTags(ctx)
	if err != nil {
		return nil, dberror.Map(err, "role tag")
	}
	tags := make([]catalogdom.RoleTag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, roleTagFromRow(row))
	}
	return tags, nil
}

func (r *Repository) writeAggregate(ctx context.Context, query *sqlc.Queries, product catalogdom.Product) error {
	id := int64(product.ID)
	if _, err := query.UpsertProductFoodClassification(ctx, classificationParams(id, product.Classification)); err != nil {
		return dberror.Map(err, "product classification")
	}
	if err := query.DeleteProductRoles(ctx, id); err != nil {
		return dberror.Map(err, "product roles")
	}
	for _, role := range product.Roles {
		if err := query.CreateProductRole(ctx, sqlc.CreateProductRoleParams{ProductID: id, RoleTagID: role.ID}); err != nil {
			return dberror.Map(err, "role tag")
		}
	}
	if err := query.DeleteProductAllergens(ctx, id); err != nil {
		return dberror.Map(err, "product allergens")
	}
	for _, allergen := range product.Allergens {
		if err := query.CreateProductAllergen(ctx, sqlc.CreateProductAllergenParams{ProductID: id, Allergen: string(allergen)}); err != nil {
			return dberror.Map(err, "product allergen")
		}
	}
	return nil
}

func (r *Repository) load(ctx context.Context, query *sqlc.Queries, id catalogdom.ProductID) (catalogdom.Product, error) {
	row, err := query.GetProduct(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product")
	}
	product, err := productFromRow(row)
	if err != nil {
		return catalogdom.Product{}, err
	}
	roles, err := query.ListProductRoleTags(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product roles")
	}
	product.Roles = make([]catalogdom.RoleTag, 0, len(roles))
	for _, role := range roles {
		product.Roles = append(product.Roles, roleTagFromRow(role))
	}
	allergens, err := query.ListProductAllergens(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product allergens")
	}
	product.Allergens = make([]catalogdom.Allergen, len(allergens))
	for i, allergen := range allergens {
		product.Allergens[i] = catalogdom.Allergen(allergen)
	}
	classification, err := query.GetProductFoodClassification(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, dberror.Map(err, "product classification")
	}
	product.Classification, err = classificationFromRow(classification)
	if err != nil {
		return catalogdom.Product{}, err
	}
	return product, nil
}
