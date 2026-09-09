-- name: LookupLLMCache :one
SELECT *
FROM llm_cache
WHERE prompt_hash = sqlc.arg(prompt_hash);

-- name: UpsertLLMCache :one
INSERT INTO llm_cache (prompt_hash, model, temperature, response)
VALUES (
    sqlc.arg(prompt_hash),
    sqlc.arg(model),
    sqlc.arg(temperature),
    sqlc.arg(response)
)
ON CONFLICT (prompt_hash) DO UPDATE SET
    model = EXCLUDED.model,
    temperature = EXCLUDED.temperature,
    response = EXCLUDED.response
RETURNING *;

-- name: CreateLLMCall :one
INSERT INTO llm_call (
    plan_id,
    provider,
    model,
    temperature,
    prompt_hash,
    prompt,
    response,
    tokens_in,
    tokens_out,
    latency_ms,
    status,
    error
) VALUES (
    sqlc.arg(plan_id),
    sqlc.arg(provider),
    sqlc.arg(model),
    sqlc.arg(temperature),
    sqlc.arg(prompt_hash),
    sqlc.arg(prompt),
    sqlc.arg(response),
    sqlc.arg(tokens_in),
    sqlc.arg(tokens_out),
    sqlc.arg(latency_ms),
    sqlc.arg(status),
    sqlc.arg(error)
)
RETURNING *;

-- name: ListLLMCalls :many
SELECT *
FROM llm_call
ORDER BY id DESC
LIMIT sqlc.arg(row_limit);
