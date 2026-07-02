-- Command-side idempotency records (plan 03 S8b). Scoped by (key, owner_sub).

-- name: GetIdempotencyRecord :one
SELECT key, owner_sub, request_hash, response_status, response_body, created_at
FROM idempotency_record
WHERE key = $1 AND owner_sub = $2;

-- name: InsertIdempotencyRecord :exec
INSERT INTO idempotency_record (key, owner_sub, request_hash, response_status, response_body)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteExpiredIdempotency :exec
DELETE FROM idempotency_record WHERE created_at < $1;
