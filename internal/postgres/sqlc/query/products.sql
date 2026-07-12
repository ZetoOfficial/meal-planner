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
    notes,
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
    sqlc.arg(brand),
    sqlc.arg(name),
    sqlc.arg(protein_per_100g),
    sqlc.arg(fat_per_100g),
    sqlc.arg(carb_per_100g),
    sqlc.arg(kcal_per_100g),
    sqlc.arg(price_per_100g),
    sqlc.arg(default_unit),
    sqlc.arg(purchasable),
    sqlc.arg(notes),
    sqlc.arg(saturated_fat_per_100g),
    sqlc.arg(trans_fat_per_100g),
    sqlc.arg(industrial_trans_fat_per_100g),
    sqlc.arg(fiber_per_100g),
    sqlc.arg(sodium_mg_per_100g),
    sqlc.arg(salt_g_per_100g),
    sqlc.arg(total_sugars_per_100g),
    sqlc.arg(added_sugars_per_100g),
    sqlc.arg(free_sugars_per_100g),
    sqlc.arg(contains_partially_hydrogenated_oils),
    sqlc.arg(nutrition_data_quality),
    sqlc.arg(nutrition_coverage_score),
    sqlc.arg(source_kind),
    sqlc.arg(source_note),
    sqlc.arg(last_verified_at)
)
RETURNING *;

-- name: GetProduct :one
SELECT
    id,
    brand,
    name,
    protein_per_100g,
    fat_per_100g,
    carb_per_100g,
    kcal_per_100g,
    price_per_100g,
    default_unit,
    purchasable,
    notes,
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
    last_verified_at,
    created_at,
    updated_at
FROM product
WHERE id = sqlc.arg(id);

-- name: ListProducts :many
SELECT *
FROM product
ORDER BY brand, name;

-- name: ListProductsByRole :many
SELECT p.*
FROM product p
JOIN product_role pr ON pr.product_id = p.id
JOIN role_tag rt ON rt.id = pr.role_tag_id
WHERE rt.code = sqlc.arg(role_code)
ORDER BY p.brand, p.name;

-- name: UpdateProduct :one
UPDATE product
SET
    brand = sqlc.arg(brand),
    name = sqlc.arg(name),
    protein_per_100g = sqlc.arg(protein_per_100g),
    fat_per_100g = sqlc.arg(fat_per_100g),
    carb_per_100g = sqlc.arg(carb_per_100g),
    kcal_per_100g = sqlc.arg(kcal_per_100g),
    price_per_100g = sqlc.arg(price_per_100g),
    default_unit = sqlc.arg(default_unit),
    purchasable = sqlc.arg(purchasable),
    notes = sqlc.arg(notes),
    saturated_fat_per_100g = sqlc.arg(saturated_fat_per_100g),
    trans_fat_per_100g = sqlc.arg(trans_fat_per_100g),
    industrial_trans_fat_per_100g = sqlc.arg(industrial_trans_fat_per_100g),
    fiber_per_100g = sqlc.arg(fiber_per_100g),
    sodium_mg_per_100g = sqlc.arg(sodium_mg_per_100g),
    salt_g_per_100g = sqlc.arg(salt_g_per_100g),
    total_sugars_per_100g = sqlc.arg(total_sugars_per_100g),
    added_sugars_per_100g = sqlc.arg(added_sugars_per_100g),
    free_sugars_per_100g = sqlc.arg(free_sugars_per_100g),
    contains_partially_hydrogenated_oils = sqlc.arg(contains_partially_hydrogenated_oils),
    nutrition_data_quality = sqlc.arg(nutrition_data_quality),
    nutrition_coverage_score = sqlc.arg(nutrition_coverage_score),
    source_kind = sqlc.arg(source_kind),
    source_note = sqlc.arg(source_note),
    last_verified_at = sqlc.arg(last_verified_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteProduct :execrows
DELETE FROM product
WHERE id = sqlc.arg(id);

-- name: GetRoleTagByCode :one
SELECT *
FROM role_tag
WHERE code = sqlc.arg(code);

-- name: ListRoleTags :many
SELECT *
FROM role_tag
ORDER BY code;

-- name: ListProductRoleTags :many
SELECT rt.*
FROM role_tag rt
JOIN product_role pr ON pr.role_tag_id = rt.id
WHERE pr.product_id = sqlc.arg(product_id)
ORDER BY rt.code;

-- name: DeleteProductRoles :exec
DELETE FROM product_role
WHERE product_id = sqlc.arg(product_id);

-- name: CreateProductRole :exec
INSERT INTO product_role (product_id, role_tag_id)
VALUES (sqlc.arg(product_id), sqlc.arg(role_tag_id));

-- name: ListProductAllergens :many
SELECT allergen
FROM product_allergen
WHERE product_id = sqlc.arg(product_id)
ORDER BY allergen;

-- name: DeleteProductAllergens :exec
DELETE FROM product_allergen
WHERE product_id = sqlc.arg(product_id);

-- name: CreateProductAllergen :exec
INSERT INTO product_allergen (product_id, allergen)
VALUES (sqlc.arg(product_id), sqlc.arg(allergen));

-- name: GetProductFoodClassification :one
SELECT *
FROM product_food_classification
WHERE product_id = sqlc.arg(product_id);

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
    classification_source,
    classification_note
) VALUES (
    sqlc.arg(product_id),
    sqlc.arg(food_group),
    sqlc.arg(processing_level),
    sqlc.arg(processing_confidence),
    sqlc.arg(counts_to_fruit_veg),
    sqlc.arg(is_legume),
    sqlc.arg(is_whole_grain),
    sqlc.arg(whole_grain_ratio),
    sqlc.arg(is_red_meat),
    sqlc.arg(is_processed_meat),
    sqlc.arg(classification_source),
    sqlc.arg(classification_note)
)
ON CONFLICT (product_id) DO UPDATE
SET
    food_group = EXCLUDED.food_group,
    processing_level = EXCLUDED.processing_level,
    processing_confidence = EXCLUDED.processing_confidence,
    counts_to_fruit_veg = EXCLUDED.counts_to_fruit_veg,
    is_legume = EXCLUDED.is_legume,
    is_whole_grain = EXCLUDED.is_whole_grain,
    whole_grain_ratio = EXCLUDED.whole_grain_ratio,
    is_red_meat = EXCLUDED.is_red_meat,
    is_processed_meat = EXCLUDED.is_processed_meat,
    classification_source = EXCLUDED.classification_source,
    classification_note = EXCLUDED.classification_note
RETURNING *;
