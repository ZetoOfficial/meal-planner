-- name: UpsertShoppingList :one
INSERT INTO shopping_list (plan_id)
VALUES (sqlc.arg(plan_id))
ON CONFLICT (plan_id) DO UPDATE SET plan_id = EXCLUDED.plan_id
RETURNING *;

-- name: GetShoppingListByPlan :one
SELECT *
FROM shopping_list
WHERE plan_id = sqlc.arg(plan_id);

-- name: ListShoppingItems :many
SELECT
    sli.id,
    sli.list_id,
    sli.product_id,
    p.name AS product_name,
    sli.total_qty_g,
    sli.unit,
    sli.est_cost,
    sli.checked
FROM shopping_list_item sli
JOIN product p ON p.id = sli.product_id
WHERE sli.list_id = sqlc.arg(list_id)
ORDER BY p.name, sli.product_id;

-- name: UpsertShoppingItem :one
INSERT INTO shopping_list_item (list_id, product_id, total_qty_g, unit, est_cost, checked)
VALUES (
    sqlc.arg(list_id),
    sqlc.arg(product_id),
    sqlc.arg(total_qty_g),
    sqlc.arg(unit),
    sqlc.arg(est_cost),
    sqlc.arg(checked)
)
ON CONFLICT (list_id, product_id) DO UPDATE SET
    total_qty_g = EXCLUDED.total_qty_g,
    unit = EXCLUDED.unit,
    est_cost = EXCLUDED.est_cost,
    checked = EXCLUDED.checked
RETURNING *;

-- name: DeleteShoppingItemsOutsideProducts :exec
DELETE FROM shopping_list_item
WHERE list_id = sqlc.arg(list_id)
  AND NOT (product_id = ANY(sqlc.arg(product_ids)::bigint[]));

-- name: SetShoppingItemChecked :one
UPDATE shopping_list_item
SET checked = sqlc.arg(checked)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: GetShoppingItem :one
SELECT
    sli.id,
    sli.list_id,
    sli.product_id,
    p.name AS product_name,
    sli.total_qty_g,
    sli.unit,
    sli.est_cost,
    sli.checked
FROM shopping_list_item sli
JOIN product p ON p.id = sli.product_id
WHERE sli.id = sqlc.arg(id);
