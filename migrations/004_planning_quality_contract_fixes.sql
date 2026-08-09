-- 004_planning_quality_contract_fixes aligns an upgraded database with the
-- planning and nutrition-quality contracts locked by Issue 43.

ALTER TABLE eater_macro_goal
    ADD CONSTRAINT eater_macro_goal_id_eater_id_key UNIQUE (id, eater_id);

ALTER TABLE fixed_meal
    DROP CONSTRAINT fixed_meal_nonneg,
    ADD COLUMN grain_g_equiv NUMERIC(7,1),
    ADD COLUMN legume_g NUMERIC(7,1),
    ADD COLUMN upf_kcal NUMERIC(7,1),
    ADD CONSTRAINT fixed_meal_nonneg CHECK (
        protein_g >= 0 AND fat_g >= 0 AND carb_g >= 0 AND kcal >= 0
        AND (saturated_fat_g IS NULL OR saturated_fat_g >= 0)
        AND (trans_fat_g IS NULL OR trans_fat_g >= 0)
        AND (fiber_g IS NULL OR fiber_g >= 0)
        AND (sodium_mg IS NULL OR sodium_mg >= 0)
        AND (added_sugars_g IS NULL OR added_sugars_g >= 0)
        AND (free_sugars_g IS NULL OR free_sugars_g >= 0)
        AND (processed_meat_g IS NULL OR processed_meat_g >= 0)
        AND (red_meat_g IS NULL OR red_meat_g >= 0)
        AND (grain_g_equiv IS NULL OR grain_g_equiv >= 0)
        AND (whole_grain_g_equiv IS NULL OR whole_grain_g_equiv >= 0)
        AND (legume_g IS NULL OR legume_g >= 0)
        AND (fruit_veg_g IS NULL OR fruit_veg_g >= 0)
        AND (upf_kcal IS NULL OR upf_kcal >= 0)
    );

-- The old CRUD path materialized an all-default classification row even when
-- the client supplied no classification. Recover unknown for that exact shape;
-- rows with explicit provenance or any positive classification stay untouched.
UPDATE product_food_classification
SET
    food_group = NULL,
    counts_to_fruit_veg = NULL,
    is_legume = NULL,
    is_whole_grain = NULL,
    is_red_meat = NULL,
    is_processed_meat = NULL
WHERE food_group = 'other'
  AND processing_level = 'unknown'
  AND processing_confidence IS NULL
  AND counts_to_fruit_veg = FALSE
  AND is_legume = FALSE
  AND is_whole_grain = FALSE
  AND whole_grain_ratio IS NULL
  AND is_red_meat = FALSE
  AND is_processed_meat = FALSE
  AND classification_source IS NULL
  AND classification_note IS NULL;

ALTER TABLE product_food_classification
    ALTER COLUMN food_group DROP DEFAULT,
    ALTER COLUMN food_group DROP NOT NULL,
    ALTER COLUMN counts_to_fruit_veg DROP DEFAULT,
    ALTER COLUMN counts_to_fruit_veg DROP NOT NULL,
    ALTER COLUMN is_legume DROP DEFAULT,
    ALTER COLUMN is_legume DROP NOT NULL,
    ALTER COLUMN is_whole_grain DROP DEFAULT,
    ALTER COLUMN is_whole_grain DROP NOT NULL,
    ALTER COLUMN is_red_meat DROP DEFAULT,
    ALTER COLUMN is_red_meat DROP NOT NULL,
    ALTER COLUMN is_processed_meat DROP DEFAULT,
    ALTER COLUMN is_processed_meat DROP NOT NULL;

ALTER TABLE plan
    DROP CONSTRAINT days_positive,
    DROP CONSTRAINT novelty_range,
    ADD CONSTRAINT plan_values CHECK (
        days > 0
        AND novelty_level BETWEEN 0 AND 3
        AND (quality_score IS NULL OR quality_score BETWEEN 0 AND 100)
        AND quality_warnings_count >= 0
    );

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM meal) THEN
        RAISE EXCEPTION 'legacy meal rows have no reconstructable component portion factors';
    END IF;
    IF EXISTS (SELECT 1 FROM day_nutrition_summary WHERE plan_id IS NULL) THEN
        RAISE EXCEPTION 'legacy day summaries without a plan cannot become plan snapshots';
    END IF;
    IF EXISTS (SELECT 1 FROM rolling_nutrition_summary) THEN
        RAISE EXCEPTION 'legacy rolling summaries have no reconstructable plan identity';
    END IF;
END
$$;

UPDATE day_nutrition_summary
SET data_coverage_score = 0
WHERE data_coverage_score IS NULL;

UPDATE rolling_nutrition_summary
SET data_coverage_score = 0
WHERE data_coverage_score IS NULL;

ALTER TABLE day_nutrition_summary
    ALTER COLUMN data_coverage_score SET NOT NULL;

ALTER TABLE rolling_nutrition_summary
    ALTER COLUMN data_coverage_score SET NOT NULL;

CREATE TABLE plan_eater_day (
    plan_id         BIGINT NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
    eater_id        BIGINT NOT NULL REFERENCES eater(id),
    plan_date       DATE NOT NULL,
    macro_goal_id   BIGINT NOT NULL,
    generated_meal_types TEXT[] NOT NULL,
    PRIMARY KEY (plan_id, eater_id, plan_date),
    FOREIGN KEY (macro_goal_id, eater_id)
        REFERENCES eater_macro_goal(id, eater_id),
    CONSTRAINT plan_eater_day_targets_nonempty CHECK (
        cardinality(generated_meal_types) > 0
        AND generated_meal_types <@ ARRAY['breakfast', 'lunch', 'dinner', 'snack']::TEXT[]
    )
);

ALTER TABLE plan_recipe
    ADD COLUMN scale_factor NUMERIC(9,4),
    ADD COLUMN expected_output_g NUMERIC(9,1);

UPDATE plan_recipe
SET
    scale_factor = 1,
    expected_output_g = NULLIF(batch_qty, 0);

ALTER TABLE plan_recipe
    ALTER COLUMN scale_factor SET NOT NULL,
    DROP CONSTRAINT plan_recipe_batch_qty_nonneg,
    DROP COLUMN batch_qty,
    ADD CONSTRAINT plan_recipe_scale_positive CHECK (
        scale_factor > 0
        AND (expected_output_g IS NULL OR expected_output_g > 0)
    );

ALTER TABLE slot_fill
    DROP CONSTRAINT slot_fill_qty_positive;

ALTER TABLE slot_fill
    RENAME COLUMN qty_g TO planned_input_qty_g;

ALTER TABLE slot_fill
    ALTER COLUMN planned_input_qty_g TYPE NUMERIC(9,1),
    ADD CONSTRAINT slot_fill_qty_positive CHECK (planned_input_qty_g > 0);

DROP INDEX idx_meal_recipe;

ALTER TABLE meal
    DROP CONSTRAINT one_source,
    DROP COLUMN plan_recipe_id,
    ADD CONSTRAINT meal_plan_eater_date_key UNIQUE (plan_id, eater_id, meal_date, meal_type),
    ADD CONSTRAINT meal_plan_eater_day_fkey
        FOREIGN KEY (plan_id, eater_id, meal_date)
        REFERENCES plan_eater_day(plan_id, eater_id, plan_date) ON DELETE CASCADE;

CREATE TABLE meal_component_allocation (
    meal_id            BIGINT NOT NULL REFERENCES meal(id) ON DELETE CASCADE,
    plan_recipe_id     BIGINT NOT NULL REFERENCES plan_recipe(id) ON DELETE CASCADE,
    portion_factor     NUMERIC(9,6) NOT NULL,
    planned_serving_g  NUMERIC(9,1),
    PRIMARY KEY (meal_id, plan_recipe_id),
    CONSTRAINT meal_component_allocation_values CHECK (
        portion_factor > 0 AND portion_factor <= 1
        AND (planned_serving_g IS NULL OR planned_serving_g > 0)
    )
);

ALTER TABLE day_nutrition_summary
    DROP CONSTRAINT day_nutrition_summary_plan_id_fkey,
    DROP CONSTRAINT day_nutrition_summary_eater_id_summary_date_basis_key,
    DROP CONSTRAINT day_summary_nonneg,
    ALTER COLUMN plan_id SET NOT NULL,
    ADD COLUMN grain_g_equiv NUMERIC(8,1),
    ADD CONSTRAINT day_nutrition_summary_plan_id_fkey
        FOREIGN KEY (plan_id) REFERENCES plan(id) ON DELETE CASCADE,
    ADD CONSTRAINT day_nutrition_summary_plan_eater_date_basis_key
        UNIQUE (plan_id, eater_id, summary_date, basis),
    ADD CONSTRAINT day_summary_nonneg CHECK (
        COALESCE(protein_g,0) >= 0 AND COALESCE(fat_g,0) >= 0 AND COALESCE(carb_g,0) >= 0
        AND COALESCE(kcal,0) >= 0 AND COALESCE(saturated_fat_g,0) >= 0
        AND COALESCE(trans_fat_g,0) >= 0 AND COALESCE(fiber_g,0) >= 0
        AND COALESCE(sodium_mg,0) >= 0 AND COALESCE(added_sugars_g,0) >= 0
        AND COALESCE(free_sugars_g,0) >= 0 AND COALESCE(fruit_veg_g,0) >= 0
        AND COALESCE(grain_g_equiv,0) >= 0 AND COALESCE(whole_grain_g_equiv,0) >= 0
        AND COALESCE(legume_g,0) >= 0 AND COALESCE(red_meat_g,0) >= 0
        AND COALESCE(processed_meat_g,0) >= 0
    );

DO $$
DECLARE
    legacy_unique_name TEXT;
BEGIN
    SELECT conname
    INTO legacy_unique_name
    FROM pg_constraint
    WHERE conrelid = 'rolling_nutrition_summary'::regclass
      AND contype = 'u';

    IF legacy_unique_name IS NULL THEN
        RAISE EXCEPTION 'legacy rolling summary unique constraint not found';
    END IF;
    EXECUTE format(
        'ALTER TABLE rolling_nutrition_summary DROP CONSTRAINT %I',
        legacy_unique_name
    );
END
$$;

ALTER TABLE rolling_nutrition_summary
    ADD COLUMN plan_id BIGINT,
    ALTER COLUMN plan_id SET NOT NULL,
    ADD CONSTRAINT rolling_nutrition_summary_plan_id_fkey
        FOREIGN KEY (plan_id) REFERENCES plan(id) ON DELETE CASCADE,
    ADD CONSTRAINT rolling_nutrition_summary_plan_eater_window_basis_key
        UNIQUE (plan_id, eater_id, as_of_date, window_days, basis);

ALTER TABLE rule_evaluation_result
    ADD COLUMN basis summary_basis NOT NULL DEFAULT 'planned';

UPDATE rule_evaluation_result
SET coverage_score = 0
WHERE coverage_score IS NULL;

ALTER TABLE rule_evaluation_result
    ALTER COLUMN basis DROP DEFAULT,
    ALTER COLUMN coverage_score SET NOT NULL,
    ADD CONSTRAINT rule_evaluation_result_identity_key
        UNIQUE NULLS NOT DISTINCT (entity_type, entity_id, eater_id, rule_code, basis),
    ADD CONSTRAINT rule_eval_coverage_ok CHECK (coverage_score BETWEEN 0 AND 1);

CREATE INDEX idx_plan_eater_day_eater_date ON plan_eater_day(eater_id, plan_date);
CREATE INDEX idx_meal_component_recipe ON meal_component_allocation(plan_recipe_id);

---- create above / drop below ----

DROP INDEX idx_meal_component_recipe;
DROP INDEX idx_plan_eater_day_eater_date;

ALTER TABLE rule_evaluation_result
    DROP CONSTRAINT rule_eval_coverage_ok,
    DROP CONSTRAINT rule_evaluation_result_identity_key,
    ALTER COLUMN coverage_score DROP NOT NULL,
    DROP COLUMN basis;

ALTER TABLE rolling_nutrition_summary
    DROP CONSTRAINT rolling_nutrition_summary_plan_eater_window_basis_key,
    DROP CONSTRAINT rolling_nutrition_summary_plan_id_fkey,
    ALTER COLUMN data_coverage_score DROP NOT NULL,
    DROP COLUMN plan_id,
    ADD CONSTRAINT rolling_nutrition_summary_eater_date_window_basis_key
        UNIQUE (eater_id, as_of_date, window_days, basis);

ALTER TABLE day_nutrition_summary
    DROP CONSTRAINT day_nutrition_summary_plan_eater_date_basis_key,
    DROP CONSTRAINT day_nutrition_summary_plan_id_fkey,
    DROP CONSTRAINT day_summary_nonneg,
    ALTER COLUMN data_coverage_score DROP NOT NULL,
    DROP COLUMN grain_g_equiv,
    ALTER COLUMN plan_id DROP NOT NULL,
    ADD CONSTRAINT day_nutrition_summary_plan_id_fkey
        FOREIGN KEY (plan_id) REFERENCES plan(id) ON DELETE SET NULL,
    ADD CONSTRAINT day_nutrition_summary_eater_id_summary_date_basis_key
        UNIQUE (eater_id, summary_date, basis),
    ADD CONSTRAINT day_summary_nonneg CHECK (
        COALESCE(protein_g,0) >= 0 AND COALESCE(fat_g,0) >= 0 AND COALESCE(carb_g,0) >= 0
        AND COALESCE(kcal,0) >= 0 AND COALESCE(saturated_fat_g,0) >= 0
        AND COALESCE(trans_fat_g,0) >= 0 AND COALESCE(fiber_g,0) >= 0
        AND COALESCE(sodium_mg,0) >= 0 AND COALESCE(added_sugars_g,0) >= 0
        AND COALESCE(free_sugars_g,0) >= 0 AND COALESCE(fruit_veg_g,0) >= 0
        AND COALESCE(whole_grain_g_equiv,0) >= 0 AND COALESCE(legume_g,0) >= 0
        AND COALESCE(red_meat_g,0) >= 0 AND COALESCE(processed_meat_g,0) >= 0
    );

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM meal m
        LEFT JOIN meal_component_allocation a ON a.meal_id = m.id
        WHERE m.fixed_meal_id IS NULL
        GROUP BY m.id
        HAVING count(a.plan_recipe_id) <> 1
    ) THEN
        RAISE EXCEPTION 'multi-component meals cannot be represented by the legacy schema';
    END IF;
END
$$;

ALTER TABLE meal
    ADD COLUMN plan_recipe_id BIGINT REFERENCES plan_recipe(id) ON DELETE SET NULL;

UPDATE meal m
SET plan_recipe_id = a.plan_recipe_id
FROM meal_component_allocation a
WHERE a.meal_id = m.id;

ALTER TABLE meal
    DROP CONSTRAINT meal_plan_eater_day_fkey,
    DROP CONSTRAINT meal_plan_eater_date_key,
    ADD CONSTRAINT one_source CHECK (
        (plan_recipe_id IS NOT NULL)::int + (fixed_meal_id IS NOT NULL)::int = 1
    );

DROP TABLE meal_component_allocation;
CREATE INDEX idx_meal_recipe ON meal(plan_recipe_id);

ALTER TABLE slot_fill
    DROP CONSTRAINT slot_fill_qty_positive;

ALTER TABLE slot_fill
    RENAME COLUMN planned_input_qty_g TO qty_g;

ALTER TABLE slot_fill
    ALTER COLUMN qty_g TYPE NUMERIC(7,1),
    ADD CONSTRAINT slot_fill_qty_positive CHECK (qty_g > 0);

ALTER TABLE plan_recipe
    DROP CONSTRAINT plan_recipe_scale_positive,
    ADD COLUMN batch_qty NUMERIC(8,1);

UPDATE plan_recipe
SET batch_qty = expected_output_g;

ALTER TABLE plan_recipe
    DROP COLUMN expected_output_g,
    DROP COLUMN scale_factor,
    ADD CONSTRAINT plan_recipe_batch_qty_nonneg CHECK (batch_qty IS NULL OR batch_qty >= 0);

DROP TABLE plan_eater_day;

ALTER TABLE plan
    DROP CONSTRAINT plan_values,
    ADD CONSTRAINT days_positive CHECK (days > 0),
    ADD CONSTRAINT novelty_range CHECK (novelty_level BETWEEN 0 AND 3);

UPDATE product_food_classification
SET
    food_group = COALESCE(food_group, 'other'),
    counts_to_fruit_veg = COALESCE(counts_to_fruit_veg, FALSE),
    is_legume = COALESCE(is_legume, FALSE),
    is_whole_grain = COALESCE(is_whole_grain, FALSE),
    is_red_meat = COALESCE(is_red_meat, FALSE),
    is_processed_meat = COALESCE(is_processed_meat, FALSE);

ALTER TABLE product_food_classification
    ALTER COLUMN food_group SET DEFAULT 'other',
    ALTER COLUMN food_group SET NOT NULL,
    ALTER COLUMN counts_to_fruit_veg SET DEFAULT FALSE,
    ALTER COLUMN counts_to_fruit_veg SET NOT NULL,
    ALTER COLUMN is_legume SET DEFAULT FALSE,
    ALTER COLUMN is_legume SET NOT NULL,
    ALTER COLUMN is_whole_grain SET DEFAULT FALSE,
    ALTER COLUMN is_whole_grain SET NOT NULL,
    ALTER COLUMN is_red_meat SET DEFAULT FALSE,
    ALTER COLUMN is_red_meat SET NOT NULL,
    ALTER COLUMN is_processed_meat SET DEFAULT FALSE,
    ALTER COLUMN is_processed_meat SET NOT NULL;

ALTER TABLE fixed_meal
    DROP CONSTRAINT fixed_meal_nonneg,
    DROP COLUMN upf_kcal,
    DROP COLUMN legume_g,
    DROP COLUMN grain_g_equiv,
    ADD CONSTRAINT fixed_meal_nonneg CHECK (
        protein_g >= 0 AND fat_g >= 0 AND carb_g >= 0 AND kcal >= 0
        AND (saturated_fat_g IS NULL OR saturated_fat_g >= 0)
        AND (trans_fat_g IS NULL OR trans_fat_g >= 0)
        AND (fiber_g IS NULL OR fiber_g >= 0)
        AND (sodium_mg IS NULL OR sodium_mg >= 0)
        AND (added_sugars_g IS NULL OR added_sugars_g >= 0)
        AND (free_sugars_g IS NULL OR free_sugars_g >= 0)
        AND (processed_meat_g IS NULL OR processed_meat_g >= 0)
        AND (red_meat_g IS NULL OR red_meat_g >= 0)
        AND (whole_grain_g_equiv IS NULL OR whole_grain_g_equiv >= 0)
        AND (fruit_veg_g IS NULL OR fruit_veg_g >= 0)
    );

ALTER TABLE eater_macro_goal
    DROP CONSTRAINT eater_macro_goal_id_eater_id_key;
