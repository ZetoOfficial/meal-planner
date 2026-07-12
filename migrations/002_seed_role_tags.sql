-- 002_seed_role_tags — stable role taxonomy for MVP recipe slots.
--
-- Seed parents before descendants. Parent ids are always selected by code so
-- this migration is safe to apply against databases whose identity values
-- differ from a fresh local installation.

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES
    ('protein', 'protein', 'Protein', NULL),
    ('carb', 'carb', 'Carbohydrate', NULL),
    ('veg', 'veg', 'Vegetable', NULL),
    ('sauce', 'sauce', 'Sauce', NULL),
    ('fat', 'fat', 'Fat', NULL),
    ('other', 'other', 'Other', NULL)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES (
    'protein:lean',
    'protein',
    'Lean protein',
    (SELECT id FROM role_tag WHERE code = 'protein')
)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES (
    'protein:lean:breast',
    'protein',
    'Lean protein: breast',
    (SELECT id FROM role_tag WHERE code = 'protein:lean')
)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES (
    'carb:grain',
    'carb',
    'Grain',
    (SELECT id FROM role_tag WHERE code = 'carb')
)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES (
    'veg:green',
    'veg',
    'Green vegetable',
    (SELECT id FROM role_tag WHERE code = 'veg')
)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

INSERT INTO role_tag (code, kind, label, parent_id)
VALUES (
    'sauce:fat',
    'sauce',
    'Fat-based sauce',
    (SELECT id FROM role_tag WHERE code = 'sauce')
)
ON CONFLICT (code) DO UPDATE
SET
    kind = EXCLUDED.kind,
    label = EXCLUDED.label,
    parent_id = EXCLUDED.parent_id;

---- create above / drop below ----

-- Role tags are durable reference data, not migration-owned rows. A database
-- may already contain these codes, and products may reference them after the
-- migration runs. Do not delete or detach either pre-existing taxonomy rows or
-- product-role assignments when tern rolls this migration back.
SELECT 1;
