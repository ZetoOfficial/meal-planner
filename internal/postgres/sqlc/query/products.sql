-- name: GetProduct :one
SELECT * FROM product WHERE id = $1;

-- name: ListProducts :many
SELECT * FROM product
ORDER BY brand, name;

-- name: ListProductsByRole :many
SELECT p.* FROM product p
JOIN product_role pr ON pr.product_id = p.id
JOIN role_tag rt ON rt.id = pr.role_tag_id
WHERE rt.code = $1
ORDER BY p.brand, p.name;

-- name: CreateProduct :one
INSERT INTO product (
    brand,
    name,
    protein_per_100g,
    fat_per_100g,
    carb_per_100g,
    kcal_per_100g,
    price_per_100g,
    default_unit,
    purchasable,
    saturated_fat_per_100g,
    trans_fat_per_100g,
    industrial_trans_fat_per_100g,
    fiber_per_100g,
    sodium_mg_per_100g,
    salt_g_per_100g,
    total_sugars_per_100g,
    added_sugars_per_100g,
    free_sugars_per_100g,
    contains_partially_hydrogenated_oils,
    nutrition_data_quality,
    nutrition_coverage_score,
    source_kind,
    source_note,
    last_verified_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13, $14, $15, $16, $17, $18,
    $19, $20, $21, $22, $23, $24
)
RETURNING *;

-- name: UpdateProduct :one
UPDATE product
SET
    brand = $2,
    name = $3,
    protein_per_100g = $4,
    fat_per_100g = $5,
    carb_per_100g = $6,
    kcal_per_100g = $7,
    price_per_100g = $8,
    default_unit = $9,
    purchasable = $10,
    saturated_fat_per_100g = $11,
    trans_fat_per_100g = $12,
    industrial_trans_fat_per_100g = $13,
    fiber_per_100g = $14,
    sodium_mg_per_100g = $15,
    salt_g_per_100g = $16,
    total_sugars_per_100g = $17,
    added_sugars_per_100g = $18,
    free_sugars_per_100g = $19,
    contains_partially_hydrogenated_oils = $20,
    nutrition_data_quality = $21,
    nutrition_coverage_score = $22,
    source_kind = $23,
    source_note = $24,
    last_verified_at = $25,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProduct :execrows
DELETE FROM product WHERE id = $1;

-- name: ListProductAllergens :many
SELECT allergen FROM product_allergen
WHERE product_id = $1
ORDER BY allergen;

-- name: DeleteProductAllergens :exec
DELETE FROM product_allergen WHERE product_id = $1;

-- name: AddProductAllergen :exec
INSERT INTO product_allergen (product_id, allergen)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListProductRoles :many
SELECT rt.code
FROM product_role pr
JOIN role_tag rt ON rt.id = pr.role_tag_id
WHERE pr.product_id = $1
ORDER BY rt.code;

-- name: DeleteProductRoles :exec
DELETE FROM product_role WHERE product_id = $1;

-- name: UpsertRoleTag :one
INSERT INTO role_tag (code, kind, label)
VALUES ($1, $2, $3)
ON CONFLICT (code) DO UPDATE
SET code = EXCLUDED.code
RETURNING id, code, kind, label, parent_id;

-- name: AddProductRole :exec
INSERT INTO product_role (product_id, role_tag_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListRoleTags :many
SELECT id, code, kind, label, parent_id
FROM role_tag
ORDER BY code;

-- name: GetProductFoodClassification :one
SELECT * FROM product_food_classification
WHERE product_id = $1;

-- name: DeleteProductFoodClassification :exec
DELETE FROM product_food_classification WHERE product_id = $1;

-- name: UpsertProductFoodClassification :one
INSERT INTO product_food_classification (
    product_id,
    food_group,
    processing_level,
    processing_confidence,
    counts_to_fruit_veg,
    is_legume,
    is_whole_grain,
    whole_grain_ratio,
    is_red_meat,
    is_processed_meat,
    classification_note
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
ON CONFLICT (product_id) DO UPDATE SET
    food_group = EXCLUDED.food_group,
    processing_level = EXCLUDED.processing_level,
    processing_confidence = EXCLUDED.processing_confidence,
    counts_to_fruit_veg = EXCLUDED.counts_to_fruit_veg,
    is_legume = EXCLUDED.is_legume,
    is_whole_grain = EXCLUDED.is_whole_grain,
    whole_grain_ratio = EXCLUDED.whole_grain_ratio,
    is_red_meat = EXCLUDED.is_red_meat,
    is_processed_meat = EXCLUDED.is_processed_meat,
    classification_note = EXCLUDED.classification_note
RETURNING *;
