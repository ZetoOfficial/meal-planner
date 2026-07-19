-- name: CreateRecipeTemplate :one
INSERT INTO recipe_template (
    name,
    status,
    source,
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
    qty,
    unit,
    position,
    optional
) VALUES (
    sqlc.arg(template_id),
    sqlc.arg(role_tag_id),
    sqlc.arg(label),
    sqlc.arg(qty),
    sqlc.arg(unit),
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
    rs.qty,
    rs.unit,
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
