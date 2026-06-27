-- name: GetProduct :one
SELECT * FROM product WHERE id = $1;

-- name: ListProducts :many
SELECT * FROM product
ORDER BY brand, name;

-- name: ListProductsByRole :many
SELECT p.* FROM product p
JOIN product_role pr ON pr.product_id = p.id
JOIN role_tag rt ON rt.id = pr.role_tag_id
WHERE rt.code = $1
ORDER BY p.brand, p.name;

-- name: CreateProduct :one
INSERT INTO product (
    brand, name, protein_per_100g, fat_per_100g, carb_per_100g, kcal_per_100g,
    price_per_100g, default_unit, purchasable
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM product WHERE id = $1;
