-- name: CreatePlan :one
INSERT INTO plan (
    label,
    start_date,
    days,
    cook_date,
    novelty_level,
    status,
    quality_score,
    quality_warnings_count,
    shopping_list_stale
) VALUES (
    sqlc.arg(label),
    sqlc.arg(start_date),
    sqlc.arg(days),
    sqlc.arg(cook_date),
    sqlc.arg(novelty_level),
    sqlc.arg(status),
    sqlc.arg(quality_score),
    sqlc.arg(quality_warnings_count),
    sqlc.arg(shopping_list_stale)
)
RETURNING *;

-- name: CreatePlanEaterDay :one
INSERT INTO plan_eater_day (
    plan_id,
    eater_id,
    plan_date,
    macro_goal_id,
    generated_meal_types
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(eater_id),
    sqlc.arg(plan_date),
    sqlc.arg(macro_goal_id),
    sqlc.arg(generated_meal_types)
)
RETURNING *;

-- name: CreatePlanRecipe :one
INSERT INTO plan_recipe (
    plan_id,
    template_id,
    label,
    scale_factor,
    expected_output_g
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(template_id),
    sqlc.arg(label),
    sqlc.arg(scale_factor),
    sqlc.arg(expected_output_g)
)
RETURNING *;

-- name: CreateSlotFill :one
INSERT INTO slot_fill (
    plan_recipe_id,
    slot_id,
    product_id,
    planned_input_qty_g
) VALUES (
    sqlc.arg(plan_recipe_id),
    sqlc.arg(slot_id),
    sqlc.arg(product_id),
    sqlc.arg(planned_input_qty_g)
)
RETURNING *;

-- name: CreateMeal :one
INSERT INTO meal (
    plan_id,
    eater_id,
    meal_date,
    meal_type,
    fixed_meal_id,
    planned_protein_g,
    planned_fat_g,
    planned_carb_g,
    planned_kcal,
    log_status
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(eater_id),
    sqlc.arg(meal_date),
    sqlc.arg(meal_type),
    sqlc.arg(fixed_meal_id),
    sqlc.arg(planned_protein_g),
    sqlc.arg(planned_fat_g),
    sqlc.arg(planned_carb_g),
    sqlc.arg(planned_kcal),
    sqlc.arg(log_status)
)
RETURNING *;

-- name: CreateMealComponentAllocation :one
INSERT INTO meal_component_allocation (
    meal_id,
    plan_recipe_id,
    portion_factor,
    planned_serving_g
) VALUES (
    sqlc.arg(meal_id),
    sqlc.arg(plan_recipe_id),
    sqlc.arg(portion_factor),
    sqlc.arg(planned_serving_g)
)
RETURNING *;

-- name: GetPlan :one
SELECT *
FROM plan
WHERE id = sqlc.arg(id);

-- name: ListPlanEaterDays :many
SELECT *
FROM plan_eater_day
WHERE plan_id = sqlc.arg(plan_id)
ORDER BY eater_id, plan_date;

-- name: ListPlanRecipes :many
SELECT *
FROM plan_recipe
WHERE plan_id = sqlc.arg(plan_id)
ORDER BY id;

-- name: ListPlanSlotFills :many
SELECT sf.*
FROM slot_fill sf
JOIN plan_recipe pr ON pr.id = sf.plan_recipe_id
WHERE pr.plan_id = sqlc.arg(plan_id)
ORDER BY sf.plan_recipe_id, sf.slot_id;

-- name: ListPlanMeals :many
SELECT *
FROM meal
WHERE plan_id = sqlc.arg(plan_id)
ORDER BY eater_id, meal_date, meal_type, id;

-- name: ListPlanMealAllocations :many
SELECT mca.*
FROM meal_component_allocation mca
JOIN meal m ON m.id = mca.meal_id
WHERE m.plan_id = sqlc.arg(plan_id)
ORDER BY mca.meal_id, mca.plan_recipe_id;

-- name: ListPlans :many
SELECT *
FROM plan
ORDER BY id DESC;

-- name: ArchivePlan :one
UPDATE plan
SET status = 'archived'
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CreatePrepDependency :one
INSERT INTO prep_dependency (plan_id, dependent_recipe_id, source_recipe_id, note)
VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(dependent_recipe_id),
    sqlc.arg(source_recipe_id),
    sqlc.arg(note)
)
ON CONFLICT (dependent_recipe_id, source_recipe_id) DO UPDATE SET note = EXCLUDED.note
RETURNING *;

-- name: ListPrepDependencies :many
SELECT *
FROM prep_dependency
WHERE plan_id = sqlc.arg(plan_id)
ORDER BY id;

-- name: UpdateSlotFillProduct :one
UPDATE slot_fill
SET product_id = sqlc.arg(product_id)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: MarkShoppingListStale :one
UPDATE plan
SET shopping_list_stale = sqlc.arg(stale)
WHERE id = sqlc.arg(id)
RETURNING *;
