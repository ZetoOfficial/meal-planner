-- name: ListPlanMealsForEater :many
SELECT *
FROM meal
WHERE plan_id = sqlc.arg(plan_id)
  AND (sqlc.narg(eater_id)::bigint IS NULL OR eater_id = sqlc.narg(eater_id)::bigint)
ORDER BY eater_id, meal_date, meal_type, id;

-- name: GetMeal :one
SELECT *
FROM meal
WHERE id = sqlc.arg(id);

-- name: LogMeal :one
UPDATE meal
SET
    log_status = sqlc.arg(log_status),
    actual_protein_g = sqlc.arg(actual_protein_g),
    actual_fat_g = sqlc.arg(actual_fat_g),
    actual_carb_g = sqlc.arg(actual_carb_g),
    actual_kcal = sqlc.arg(actual_kcal),
    cooked_by = COALESCE(sqlc.narg(cooked_by), cooked_by),
    cooked_at = CASE WHEN sqlc.narg(cooked_by) IS NULL THEN cooked_at ELSE sqlc.arg(logged_at) END,
    logged_by = sqlc.arg(logged_by),
    logged_at = sqlc.arg(logged_at)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListMealsForEaterPeriod :many
-- Приёмы едока за период — по одному плану на день. День ведёт план, в котором
-- по нему уже что-то залогировано; иначе самый свежий неархивный. Так черновики
-- на те же даты и заменённые ротации не удваивают аналитику, а история
-- архивного плана, который успели прожить, не теряется.
WITH governing AS (
    SELECT DISTINCT ON (gm.meal_date) gm.meal_date, gm.plan_id
    FROM meal gm
    JOIN plan gp ON gp.id = gm.plan_id
    WHERE gm.eater_id = sqlc.arg(eater_id)
      AND gm.meal_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
    GROUP BY gm.meal_date, gm.plan_id, gp.status
    HAVING bool_or(gm.log_status <> 'planned') OR gp.status <> 'archived'
    ORDER BY gm.meal_date, bool_or(gm.log_status <> 'planned') DESC, gm.plan_id DESC
)
SELECT m.*
FROM meal m
JOIN governing g ON g.meal_date = m.meal_date AND g.plan_id = m.plan_id
WHERE m.eater_id = sqlc.arg(eater_id)
ORDER BY m.meal_date, m.meal_type, m.id;

-- name: CreateFeedbackEvent :one
INSERT INTO feedback_event (eater_id, target_type, target_id, rating, comment, weight_delta)
VALUES (
    sqlc.arg(eater_id),
    sqlc.arg(target_type),
    sqlc.arg(target_id),
    sqlc.arg(rating),
    sqlc.arg(comment),
    sqlc.arg(weight_delta)
)
RETURNING *;

-- name: AdjustRecipeWeight :one
UPDATE recipe_template
SET weight = LEAST(GREATEST(weight + sqlc.arg(delta), sqlc.arg(minimum)), sqlc.arg(maximum)),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListTopRecipes :many
SELECT
    rt.id,
    rt.name,
    rt.cooked_count,
    count(pr.id) AS plan_usage
FROM recipe_template rt
LEFT JOIN plan_recipe pr ON pr.template_id = rt.id
GROUP BY rt.id, rt.name, rt.cooked_count
ORDER BY rt.cooked_count DESC, plan_usage DESC, rt.id
LIMIT sqlc.arg(row_limit);

-- name: ListEaterDaySummariesForPeriod :many
SELECT *
FROM day_nutrition_summary
WHERE eater_id = sqlc.arg(eater_id)
  AND basis = sqlc.arg(basis)
  AND summary_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
ORDER BY summary_date;

-- name: ListEaterRollingSummariesForPeriod :many
SELECT *
FROM rolling_nutrition_summary
WHERE eater_id = sqlc.arg(eater_id)
  AND basis = sqlc.arg(basis)
  AND as_of_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
ORDER BY as_of_date;

-- name: ListPlansForEaterPeriod :many
SELECT DISTINCT p.*
FROM plan p
JOIN plan_eater_day ped ON ped.plan_id = p.id
WHERE ped.eater_id = sqlc.arg(eater_id)
  AND ped.plan_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
ORDER BY p.id;

-- name: ListProductFrequency :many
-- Частота продуктов в запланированных и съеденных приёмах едока: вход продукта
-- делится между приёмами по доле партии, пропущенные приёмы не считаются.
-- День ведёт один план — тот же выбор, что в ListMealsForEaterPeriod.
WITH governing AS (
    SELECT DISTINCT ON (gm.meal_date) gm.meal_date, gm.plan_id
    FROM meal gm
    JOIN plan gp ON gp.id = gm.plan_id
    WHERE gm.eater_id = sqlc.arg(eater_id)
      AND gm.meal_date BETWEEN sqlc.arg(from_date) AND sqlc.arg(to_date)
    GROUP BY gm.meal_date, gm.plan_id, gp.status
    HAVING bool_or(gm.log_status <> 'planned') OR gp.status <> 'archived'
    ORDER BY gm.meal_date, bool_or(gm.log_status <> 'planned') DESC, gm.plan_id DESC
)
SELECT
    pr.id AS product_id,
    pr.brand,
    pr.name,
    count(DISTINCT m.id) AS meals_count,
    sum(sf.planned_input_qty_g * mca.portion_factor)::float8 AS total_qty_g
FROM meal m
JOIN meal_component_allocation mca ON mca.meal_id = m.id
JOIN slot_fill sf ON sf.plan_recipe_id = mca.plan_recipe_id
JOIN product pr ON pr.id = sf.product_id
JOIN governing g ON g.meal_date = m.meal_date AND g.plan_id = m.plan_id
WHERE m.eater_id = sqlc.arg(eater_id)
  AND m.log_status <> 'skipped'
GROUP BY pr.id, pr.brand, pr.name
ORDER BY meals_count DESC, total_qty_g DESC, pr.id
LIMIT sqlc.arg(row_limit);
