-- 003_recipe_batch_contract — migrate the existing recipe library to the
-- independently prepared batch-component contract locked by Issue 43.

CREATE TYPE component_kind AS ENUM (
    'protein',
    'carb_side',
    'vegetable_side',
    'sauce',
    'mixed_main'
);

ALTER TABLE recipe_template
    ADD COLUMN component_kind component_kind,
    ADD COLUMN allowed_meal_types TEXT[],
    ADD COLUMN base_servings INTEGER,
    ADD COLUMN expected_output_g NUMERIC(8,1);

-- Legacy templates represented whole dishes. Treating them as one mixed_main
-- serving preserves their proportional semantics without guessing independent portions.
UPDATE recipe_template
SET
    component_kind = 'mixed_main',
    allowed_meal_types = ARRAY['lunch', 'dinner']::TEXT[],
    base_servings = 1;

ALTER TABLE recipe_template
    ALTER COLUMN component_kind SET NOT NULL,
    ALTER COLUMN allowed_meal_types SET NOT NULL,
    ALTER COLUMN base_servings SET NOT NULL,
    DROP CONSTRAINT weight_nonneg,
    ADD CONSTRAINT recipe_template_values CHECK (
        weight >= 0
        AND base_servings > 0
        AND cardinality(allowed_meal_types) > 0
        AND allowed_meal_types <@ ARRAY['breakfast', 'lunch', 'dinner', 'snack']::TEXT[]
        AND (expected_output_g IS NULL OR expected_output_g > 0)
    );

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM recipe_slot
        WHERE lower(btrim(unit)) <> 'g'
    ) THEN
        RAISE EXCEPTION 'recipe slots with non-gram units require an explicit conversion before migration';
    END IF;
END
$$;

ALTER TABLE recipe_slot
    DROP CONSTRAINT recipe_slot_qty_positive;

ALTER TABLE recipe_slot
    RENAME COLUMN qty TO qty_g;

ALTER TABLE recipe_slot
    DROP COLUMN unit,
    ADD COLUMN prepared_qty_factor NUMERIC(6,3),
    ADD CONSTRAINT recipe_slot_qty_positive CHECK (
        qty_g > 0
        AND (prepared_qty_factor IS NULL OR prepared_qty_factor > 0)
    );

---- create above / drop below ----

ALTER TABLE recipe_slot
    DROP CONSTRAINT recipe_slot_qty_positive,
    ADD COLUMN unit TEXT NOT NULL DEFAULT 'g',
    DROP COLUMN prepared_qty_factor;

ALTER TABLE recipe_slot
    RENAME COLUMN qty_g TO qty;

ALTER TABLE recipe_slot
    ADD CONSTRAINT recipe_slot_qty_positive CHECK (qty > 0);

ALTER TABLE recipe_template
    DROP CONSTRAINT recipe_template_values,
    DROP COLUMN expected_output_g,
    DROP COLUMN base_servings,
    DROP COLUMN allowed_meal_types,
    DROP COLUMN component_kind,
    ADD CONSTRAINT weight_nonneg CHECK (weight >= 0);

DROP TYPE component_kind;
