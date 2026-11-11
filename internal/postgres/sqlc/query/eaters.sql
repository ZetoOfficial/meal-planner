-- name: CreateEater :one
INSERT INTO eater (name, sex)
VALUES ($1, $2)
RETURNING *;

-- name: ListEaters :many
SELECT * FROM eater ORDER BY name;

-- name: GetEater :one
SELECT * FROM eater WHERE id = $1;

-- name: UpdateEater :one
UPDATE eater
SET name = $2, sex = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListEaterAllergens :many
SELECT allergen FROM eater_allergen
WHERE eater_id = $1
ORDER BY allergen;

-- name: DeleteEaterAllergens :exec
DELETE FROM eater_allergen WHERE eater_id = $1;

-- name: CreateEaterAllergen :exec
INSERT INTO eater_allergen (eater_id, allergen)
VALUES ($1, $2);

-- name: CreateMacroGoal :one
INSERT INTO eater_macro_goal (
    eater_id, effective_from,
    protein_min_g, protein_max_g, fat_min_g, fat_max_g,
    carb_min_g, carb_max_g, kcal_min, kcal_max, note
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetCurrentMacroGoal :one
SELECT * FROM eater_macro_goal
WHERE eater_id = $1 AND effective_from <= CURRENT_DATE
ORDER BY effective_from DESC
LIMIT 1;

-- name: ListMacroGoals :many
SELECT * FROM eater_macro_goal
WHERE eater_id = $1
ORDER BY effective_from DESC;

-- name: CreateFixedMeal :one
INSERT INTO fixed_meal (
    eater_id, meal_type, label, protein_g, fat_g, carb_g, kcal,
    saturated_fat_g, trans_fat_g, fiber_g, sodium_mg, added_sugars_g,
    free_sugars_g, processed_meat_g, red_meat_g, whole_grain_g_equiv,
    grain_g_equiv, legume_g, fruit_veg_g, upf_kcal,
    include_in_analytics, include_in_warnings, active
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23
)
RETURNING *;

-- name: ListActiveFixedMeals :many
SELECT * FROM fixed_meal
WHERE eater_id = $1 AND active = TRUE
ORDER BY meal_type;

-- name: CreateEaterExclusion :one
INSERT INTO eater_exclusion (eater_id, target_kind, target_value, severity, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEaterExclusions :many
SELECT * FROM eater_exclusion
WHERE eater_id = $1
ORDER BY id;

-- name: DeleteEaterExclusion :one
DELETE FROM eater_exclusion
WHERE id = $1
RETURNING *;

-- name: RoleTagExists :one
SELECT EXISTS(SELECT 1 FROM role_tag WHERE code = $1);

-- name: ProductExists :one
SELECT EXISTS(SELECT 1 FROM product WHERE id = $1);

-- name: DeleteEater :one
DELETE FROM eater
WHERE id = $1
RETURNING id;

-- name: GetFixedMeal :one
SELECT * FROM fixed_meal WHERE id = $1;

-- name: UpdateFixedMeal :one
UPDATE fixed_meal
SET meal_type = $2, label = $3, protein_g = $4, fat_g = $5, carb_g = $6, kcal = $7,
    saturated_fat_g = $8, trans_fat_g = $9, fiber_g = $10, sodium_mg = $11,
    added_sugars_g = $12, free_sugars_g = $13, processed_meat_g = $14, red_meat_g = $15,
    whole_grain_g_equiv = $16, grain_g_equiv = $17, legume_g = $18, fruit_veg_g = $19,
    upf_kcal = $20, include_in_analytics = $21, include_in_warnings = $22
WHERE id = $1 AND active = TRUE
RETURNING *;

-- name: DeactivateFixedMeal :one
UPDATE fixed_meal
SET active = FALSE
WHERE id = $1 AND active = TRUE
RETURNING id;
