-- dev_seed — local demo data for the owner-only MVP.
--
-- This is NOT a migration: it is development data, applied on demand with
-- `make seed`. It is idempotent, so re-running it refreshes the demo state
-- instead of duplicating it. Never apply it to a database that holds real
-- planning history.
--
-- The fixture is deliberately small but complete enough to exercise the whole
-- path: two eaters with different goals and constraints, a fixed breakfast, a
-- product set with mixed nutrition coverage (so quality warnings and unknown
-- verdicts both appear), and four permanent recipes that a three-day plan can
-- actually be built from.

BEGIN;

-- ---------------------------------------------------------------- eaters

INSERT INTO eater (name, sex)
VALUES ('Pasha', 'male'), ('Sasha', 'female')
ON CONFLICT DO NOTHING;

INSERT INTO eater_macro_goal (
    eater_id, effective_from,
    protein_min_g, protein_max_g, fat_min_g, fat_max_g,
    carb_min_g, carb_max_g, kcal_min, kcal_max, note
)
SELECT e.id, CURRENT_DATE - 30, 150, 210, 60, 100, 200, 320, 2200, 2800, 'demo goal'
FROM eater e WHERE e.name = 'Pasha'
ON CONFLICT DO NOTHING;

INSERT INTO eater_macro_goal (
    eater_id, effective_from,
    protein_min_g, protein_max_g, fat_min_g, fat_max_g,
    carb_min_g, carb_max_g, kcal_min, kcal_max, note
)
SELECT e.id, CURRENT_DATE - 30, 100, 150, 45, 80, 150, 250, 1700, 2100, 'demo goal'
FROM eater e WHERE e.name = 'Sasha'
ON CONFLICT DO NOTHING;

-- Sasha avoids milk; the seeded products and recipes below respect that.
INSERT INTO eater_allergen (eater_id, allergen)
SELECT e.id, 'milk' FROM eater e WHERE e.name = 'Sasha'
ON CONFLICT DO NOTHING;

INSERT INTO eater_exclusion (eater_id, target_kind, target_value, severity, note)
SELECT e.id, 'food_group', 'confectionery', 'hard', 'demo exclusion'
FROM eater e WHERE e.name = 'Pasha'
ON CONFLICT DO NOTHING;

-- Sasha eats the same breakfast every day; the solver plans around it.
INSERT INTO fixed_meal (
    eater_id, meal_type, label,
    protein_g, fat_g, carb_g, kcal,
    fiber_g, sodium_mg, saturated_fat_g, added_sugars_g, fruit_veg_g,
    include_in_analytics, include_in_warnings, active
)
SELECT e.id, 'breakfast', 'Oats with berries',
       14, 9, 55, 370,
       8, 60, 2, 4, 80,
       TRUE, TRUE, TRUE
FROM eater e WHERE e.name = 'Sasha'
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------- products

INSERT INTO product (
    brand, name, protein_per_100g, fat_per_100g, carb_per_100g, kcal_per_100g,
    price_per_100g, default_unit, purchasable,
    saturated_fat_per_100g, fiber_per_100g, sodium_mg_per_100g, added_sugars_per_100g,
    nutrition_data_quality, source_kind
)
VALUES
    ('Troekurovo', 'Chicken breast', 23.0, 2.0, 0.0, 110.0, 45.00, 'g', TRUE,
     0.6, 0.0, 60.0, 0.0, 'label_exact', 'pack_label'),
    ('Agama', 'Cod fillet', 18.0, 0.7, 0.0, 82.0, 70.00, 'g', TRUE,
     0.2, 0.0, 70.0, 0.0, 'label_exact', 'pack_label'),
    ('Mistral', 'Brown rice', 7.5, 2.2, 72.0, 350.0, 12.00, 'g', TRUE,
     0.5, 3.4, 5.0, 0.0, 'label_exact', 'pack_label'),
    ('Mistral', 'Green lentils', 24.0, 1.5, 46.0, 310.0, 18.00, 'g', TRUE,
     0.2, 11.0, 6.0, 0.0, 'label_exact', 'pack_label'),
    ('Local', 'Broccoli', 2.8, 0.4, 5.0, 34.0, 30.00, 'g', TRUE,
     0.1, 2.6, 33.0, 0.0, 'trusted_db', 'composition_table'),
    ('Local', 'Carrot', 0.9, 0.2, 9.6, 41.0, 8.00, 'g', TRUE,
     0.0, 2.8, 69.0, 0.0, 'trusted_db', 'composition_table'),
    ('Iberica', 'Olive oil', 0.0, 100.0, 0.0, 884.0, 90.00, 'ml', TRUE,
     14.0, 0.0, 2.0, 0.0, 'label_exact', 'pack_label'),
    ('Agama', 'Salmon fillet', 20.0, 13.0, 0.0, 200.0, 130.00, 'g', TRUE,
     2.5, 0.0, 60.0, 0.0, 'label_exact', 'pack_label'),
    -- Deliberately sparse: this product exercises unknown quality coverage.
    ('Market', 'Bakery flatbread', 8.0, 3.0, 50.0, 270.0, 20.00, 'g', TRUE,
     NULL, NULL, NULL, NULL, 'unknown', 'manual_estimate')
ON CONFLICT (brand, name) DO UPDATE SET
    protein_per_100g = EXCLUDED.protein_per_100g,
    fat_per_100g = EXCLUDED.fat_per_100g,
    carb_per_100g = EXCLUDED.carb_per_100g,
    kcal_per_100g = EXCLUDED.kcal_per_100g,
    price_per_100g = EXCLUDED.price_per_100g,
    saturated_fat_per_100g = EXCLUDED.saturated_fat_per_100g,
    fiber_per_100g = EXCLUDED.fiber_per_100g,
    sodium_mg_per_100g = EXCLUDED.sodium_mg_per_100g,
    added_sugars_per_100g = EXCLUDED.added_sugars_per_100g,
    nutrition_data_quality = EXCLUDED.nutrition_data_quality,
    source_kind = EXCLUDED.source_kind;

INSERT INTO product_food_classification (
    product_id, food_group, processing_level, counts_to_fruit_veg,
    is_legume, is_whole_grain, whole_grain_ratio, is_red_meat, is_processed_meat
)
SELECT p.id, v.food_group::food_group, v.processing_level::processing_level,
       v.counts_to_fruit_veg::boolean, v.is_legume::boolean, v.is_whole_grain::boolean,
       v.whole_grain_ratio, v.is_red_meat::boolean, v.is_processed_meat::boolean
FROM (VALUES
    ('Troekurovo', 'Chicken breast', 'white_meat', 'minimally_processed', FALSE, FALSE, FALSE, NULL::numeric, FALSE, FALSE),
    ('Agama', 'Cod fillet', 'fish_seafood', 'minimally_processed', FALSE, FALSE, FALSE, NULL, FALSE, FALSE),
    ('Mistral', 'Brown rice', 'grain', 'minimally_processed', FALSE, FALSE, TRUE, 1.000, FALSE, FALSE),
    ('Mistral', 'Green lentils', 'legume', 'minimally_processed', FALSE, TRUE, FALSE, NULL, FALSE, FALSE),
    ('Local', 'Broccoli', 'vegetable', 'minimally_processed', TRUE, FALSE, FALSE, NULL, FALSE, FALSE),
    ('Local', 'Carrot', 'vegetable', 'minimally_processed', TRUE, FALSE, FALSE, NULL, FALSE, FALSE),
    ('Iberica', 'Olive oil', 'other', 'processed_culinary', FALSE, FALSE, FALSE, NULL, FALSE, FALSE),
    ('Agama', 'Salmon fillet', 'fish_seafood', 'minimally_processed', FALSE, FALSE, FALSE, NULL, FALSE, FALSE),
    -- Deliberately unclassified beyond its group: exercises unknown quality.
    ('Market', 'Bakery flatbread', 'grain', 'processed_food', NULL, NULL, NULL, NULL, NULL, NULL)
) AS v(brand, name, food_group, processing_level, counts_to_fruit_veg, is_legume, is_whole_grain, whole_grain_ratio, is_red_meat, is_processed_meat)
JOIN product p ON p.brand = v.brand AND p.name = v.name
ON CONFLICT (product_id) DO UPDATE SET
    food_group = EXCLUDED.food_group,
    processing_level = EXCLUDED.processing_level,
    counts_to_fruit_veg = EXCLUDED.counts_to_fruit_veg,
    is_legume = EXCLUDED.is_legume,
    is_whole_grain = EXCLUDED.is_whole_grain,
    whole_grain_ratio = EXCLUDED.whole_grain_ratio,
    is_red_meat = EXCLUDED.is_red_meat,
    is_processed_meat = EXCLUDED.is_processed_meat;

INSERT INTO product_role (product_id, role_tag_id)
SELECT p.id, rt.id
FROM (VALUES
    ('Troekurovo', 'Chicken breast', 'protein:lean:breast'),
    ('Agama', 'Cod fillet', 'protein:lean'),
    ('Mistral', 'Brown rice', 'carb:grain'),
    ('Mistral', 'Green lentils', 'protein:lean'),
    ('Local', 'Broccoli', 'veg:green'),
    ('Local', 'Carrot', 'veg:green'),
    ('Iberica', 'Olive oil', 'fat'),
    ('Agama', 'Salmon fillet', 'protein:lean'),
    ('Market', 'Bakery flatbread', 'carb:grain')
) AS v(brand, name, role_code)
JOIN product p ON p.brand = v.brand AND p.name = v.name
JOIN role_tag rt ON rt.code = v.role_code
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------- recipes

INSERT INTO recipe_template (
    name, status, source, component_kind, allowed_meal_types,
    base_servings, expected_output_g, cuisine, cooking_method,
    storage, equipment, instructions, weight
)
VALUES
    ('Roast chicken batch', 'permanent', 'manual', 'protein',
     ARRAY['lunch', 'dinner'], 4, 720, 'Mediterranean', 'roast',
     'keeps_3d_reheat', ARRAY['oven'], 'Roast, portion and chill.', 1.0),
    ('Baked cod batch', 'permanent', 'manual', 'protein',
     ARRAY['lunch', 'dinner'], 4, 640, 'Nordic', 'bake',
     'keeps_3d', ARRAY['oven'], 'Bake, portion and chill.', 1.0),
    ('Brown rice batch', 'permanent', 'manual', 'carb_side',
     ARRAY['lunch', 'dinner'], 4, 900, NULL, 'boil',
     'keeps_3d_reheat', ARRAY['stove'], 'Boil and cool quickly.', 1.0),
    ('Roast vegetables batch', 'permanent', 'manual', 'vegetable_side',
     ARRAY['lunch', 'dinner'], 4, 700, NULL, 'roast',
     'keeps_3d', ARRAY['oven'], 'Roast until tender.', 1.0),
    ('Lentil stew batch', 'permanent', 'manual', 'mixed_main',
     ARRAY['lunch', 'dinner'], 4, 1000, NULL, 'stew',
     'keeps_3d_reheat', ARRAY['stove'], 'Simmer until soft.', 1.0)
ON CONFLICT DO NOTHING;

INSERT INTO recipe_slot (template_id, role_tag_id, label, qty_g, prepared_qty_factor, position, optional)
SELECT rt.id, tag.id, v.label, v.qty_g, v.prepared_qty_factor, v.position, v.optional
FROM (VALUES
    ('Roast chicken batch', 'protein:lean:breast', 'Chicken', 1000.0, 0.75, 0, FALSE),
    ('Roast chicken batch', 'fat', 'Olive oil', 60.0, 1.00, 1, TRUE),
    ('Baked cod batch', 'protein:lean', 'Cod', 900.0, 0.85, 0, FALSE),
    ('Brown rice batch', 'carb:grain', 'Rice', 320.0, 2.60, 0, FALSE),
    ('Roast vegetables batch', 'veg:green', 'Vegetables', 800.0, 0.85, 0, FALSE),
    ('Roast vegetables batch', 'fat', 'Olive oil', 60.0, 1.00, 1, TRUE),
    ('Lentil stew batch', 'protein:lean', 'Lentils', 400.0, 2.40, 0, FALSE),
    ('Lentil stew batch', 'veg:green', 'Vegetables', 400.0, 0.85, 1, FALSE)
) AS v(recipe_name, role_code, label, qty_g, prepared_qty_factor, position, optional)
JOIN recipe_template rt ON rt.name = v.recipe_name
JOIN role_tag tag ON tag.code = v.role_code
ON CONFLICT DO NOTHING;

COMMIT;
