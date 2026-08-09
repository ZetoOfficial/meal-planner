-- name: CreateRecipeTemplate :one
INSERT INTO recipe_template (
    name,
    status,
    source,
    component_kind,
    allowed_meal_types,
    base_servings,
    expected_output_g,
    cuisine,
    cooking_method,
    storage,
    storage_notes,
    equipment,
    instructions,
    weight,
    cooked_count
) VALUES (
    sqlc.arg(name),
    sqlc.arg(status),
    sqlc.arg(source),
    sqlc.arg(component_kind),
    sqlc.arg(allowed_meal_types),
    sqlc.arg(base_servings),
    sqlc.arg(expected_output_g),
    sqlc.arg(cuisine),
    sqlc.arg(cooking_method),
    sqlc.arg(storage),
    sqlc.arg(storage_notes),
    sqlc.arg(equipment),
    sqlc.arg(instructions),
    sqlc.arg(weight),
    sqlc.arg(cooked_count)
)
RETURNING *;

-- name: CreateRecipeSlot :one
INSERT INTO recipe_slot (
    template_id,
    role_tag_id,
    label,
    qty_g,
    prepared_qty_factor,
    position,
    optional
) VALUES (
    sqlc.arg(template_id),
    sqlc.arg(role_tag_id),
    sqlc.arg(label),
    sqlc.arg(qty_g),
    sqlc.arg(prepared_qty_factor),
    sqlc.arg(position),
    sqlc.arg(optional)
)
RETURNING *;

-- name: GetRecipeTemplate :one
SELECT *
FROM recipe_template
WHERE id = sqlc.arg(id);

-- name: ListRecipeTemplates :many
SELECT *
FROM recipe_template
ORDER BY id;

-- name: ListRecipeTemplatesByStatus :many
SELECT *
FROM recipe_template
WHERE status = sqlc.arg(status)
ORDER BY id;

-- name: ListRecipeSlotsByTemplate :many
SELECT
    rs.id,
    rs.template_id,
    rs.role_tag_id,
    rt.code AS role_code,
    rs.label,
    rs.qty_g,
    rs.prepared_qty_factor,
    rs.position,
    rs.optional
FROM recipe_slot rs
JOIN role_tag rt ON rt.id = rs.role_tag_id
WHERE rs.template_id = sqlc.arg(template_id)
ORDER BY rs.position;

-- name: ResolveRecipeRoleTags :many
SELECT *
FROM role_tag
WHERE code = ANY(sqlc.arg(codes)::text[])
ORDER BY code;

-- name: PromoteRecipeTemplate :one
UPDATE recipe_template
SET
    status = 'permanent',
    promoted_at = sqlc.arg(promoted_at),
    cooked_count = cooked_count + 1,
    updated_at = sqlc.arg(promoted_at)
WHERE id = sqlc.arg(id)
  AND status = 'draft'
RETURNING *;
