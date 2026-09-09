-- name: UpsertDayNutritionSummary :one
INSERT INTO day_nutrition_summary (
    plan_id,
    eater_id,
    summary_date,
    basis,
    protein_g,
    fat_g,
    carb_g,
    kcal,
    saturated_fat_g,
    trans_fat_g,
    fiber_g,
    sodium_mg,
    added_sugars_g,
    free_sugars_g,
    fruit_veg_g,
    grain_g_equiv,
    whole_grain_g_equiv,
    legume_g,
    red_meat_g,
    processed_meat_g,
    upf_kcal_share_est,
    data_coverage_score
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(eater_id),
    sqlc.arg(summary_date),
    sqlc.arg(basis),
    sqlc.arg(protein_g),
    sqlc.arg(fat_g),
    sqlc.arg(carb_g),
    sqlc.arg(kcal),
    sqlc.arg(saturated_fat_g),
    sqlc.arg(trans_fat_g),
    sqlc.arg(fiber_g),
    sqlc.arg(sodium_mg),
    sqlc.arg(added_sugars_g),
    sqlc.arg(free_sugars_g),
    sqlc.arg(fruit_veg_g),
    sqlc.arg(grain_g_equiv),
    sqlc.arg(whole_grain_g_equiv),
    sqlc.arg(legume_g),
    sqlc.arg(red_meat_g),
    sqlc.arg(processed_meat_g),
    sqlc.arg(upf_kcal_share_est),
    sqlc.arg(data_coverage_score)
)
ON CONFLICT (plan_id, eater_id, summary_date, basis) DO UPDATE SET
    protein_g = EXCLUDED.protein_g,
    fat_g = EXCLUDED.fat_g,
    carb_g = EXCLUDED.carb_g,
    kcal = EXCLUDED.kcal,
    saturated_fat_g = EXCLUDED.saturated_fat_g,
    trans_fat_g = EXCLUDED.trans_fat_g,
    fiber_g = EXCLUDED.fiber_g,
    sodium_mg = EXCLUDED.sodium_mg,
    added_sugars_g = EXCLUDED.added_sugars_g,
    free_sugars_g = EXCLUDED.free_sugars_g,
    fruit_veg_g = EXCLUDED.fruit_veg_g,
    grain_g_equiv = EXCLUDED.grain_g_equiv,
    whole_grain_g_equiv = EXCLUDED.whole_grain_g_equiv,
    legume_g = EXCLUDED.legume_g,
    red_meat_g = EXCLUDED.red_meat_g,
    processed_meat_g = EXCLUDED.processed_meat_g,
    upf_kcal_share_est = EXCLUDED.upf_kcal_share_est,
    data_coverage_score = EXCLUDED.data_coverage_score
RETURNING *;

-- name: UpsertRollingNutritionSummary :one
INSERT INTO rolling_nutrition_summary (
    plan_id,
    eater_id,
    as_of_date,
    window_days,
    basis,
    red_meat_g,
    processed_meat_g,
    legume_servings,
    fiber_g_avg,
    sodium_mg_avg,
    saturated_fat_g_avg,
    added_sugars_g_avg,
    fruit_veg_g_avg,
    data_coverage_score
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(eater_id),
    sqlc.arg(as_of_date),
    sqlc.arg(window_days),
    sqlc.arg(basis),
    sqlc.arg(red_meat_g),
    sqlc.arg(processed_meat_g),
    sqlc.arg(legume_servings),
    sqlc.arg(fiber_g_avg),
    sqlc.arg(sodium_mg_avg),
    sqlc.arg(saturated_fat_g_avg),
    sqlc.arg(added_sugars_g_avg),
    sqlc.arg(fruit_veg_g_avg),
    sqlc.arg(data_coverage_score)
)
ON CONFLICT (plan_id, eater_id, as_of_date, window_days, basis) DO UPDATE SET
    red_meat_g = EXCLUDED.red_meat_g,
    processed_meat_g = EXCLUDED.processed_meat_g,
    legume_servings = EXCLUDED.legume_servings,
    fiber_g_avg = EXCLUDED.fiber_g_avg,
    sodium_mg_avg = EXCLUDED.sodium_mg_avg,
    saturated_fat_g_avg = EXCLUDED.saturated_fat_g_avg,
    added_sugars_g_avg = EXCLUDED.added_sugars_g_avg,
    fruit_veg_g_avg = EXCLUDED.fruit_veg_g_avg,
    data_coverage_score = EXCLUDED.data_coverage_score
RETURNING *;

-- name: UpsertRuleEvaluationResult :one
INSERT INTO rule_evaluation_result (
    entity_type,
    entity_id,
    eater_id,
    rule_code,
    basis,
    status,
    actual_value,
    target_value,
    unit,
    eval_date,
    window_start,
    window_end,
    coverage_score,
    message
) VALUES (
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(eater_id),
    sqlc.arg(rule_code),
    sqlc.arg(basis),
    sqlc.arg(status),
    sqlc.arg(actual_value),
    sqlc.arg(target_value),
    sqlc.arg(unit),
    sqlc.arg(eval_date),
    sqlc.arg(window_start),
    sqlc.arg(window_end),
    sqlc.arg(coverage_score),
    sqlc.arg(message)
)
ON CONFLICT (entity_type, entity_id, eater_id, rule_code, basis) DO UPDATE SET
    status = EXCLUDED.status,
    actual_value = EXCLUDED.actual_value,
    target_value = EXCLUDED.target_value,
    unit = EXCLUDED.unit,
    eval_date = EXCLUDED.eval_date,
    window_start = EXCLUDED.window_start,
    window_end = EXCLUDED.window_end,
    coverage_score = EXCLUDED.coverage_score,
    message = EXCLUDED.message,
    evaluated_at = now()
RETURNING *;

-- name: ListPlanDaySummaries :many
SELECT *
FROM day_nutrition_summary
WHERE plan_id = sqlc.arg(plan_id)
  AND basis = sqlc.arg(basis)
ORDER BY eater_id, summary_date;

-- name: ListPlanRollingSummaries :many
SELECT *
FROM rolling_nutrition_summary
WHERE plan_id = sqlc.arg(plan_id)
  AND basis = sqlc.arg(basis)
ORDER BY eater_id, as_of_date;

-- name: ListRuleEvaluationResults :many
SELECT *
FROM rule_evaluation_result
WHERE entity_type = sqlc.arg(entity_type)
  AND entity_id = ANY(sqlc.arg(entity_ids)::bigint[])
ORDER BY entity_id, rule_code;
