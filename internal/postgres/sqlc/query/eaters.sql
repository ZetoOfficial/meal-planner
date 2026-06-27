-- name: ListEaters :many
SELECT * FROM eater ORDER BY name;

-- name: GetEater :one
SELECT * FROM eater WHERE id = $1;

-- name: GetCurrentMacroGoal :one
SELECT * FROM eater_macro_goal
WHERE eater_id = $1 AND effective_from <= CURRENT_DATE
ORDER BY effective_from DESC
LIMIT 1;

-- name: ListMacroGoals :many
SELECT * FROM eater_macro_goal
WHERE eater_id = $1
ORDER BY effective_from DESC;

-- name: ListActiveFixedMeals :many
SELECT * FROM fixed_meal
WHERE eater_id = $1 AND active = TRUE
ORDER BY meal_type;
