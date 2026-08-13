-- name: CreateToken :one
INSERT INTO token (label, token_hash, token_prefix)
VALUES (?, ?, ?)
RETURNING id;

-- name: GetToken :one
SELECT * FROM token WHERE id = ?;

-- name: GetTokenByHash :one
SELECT * FROM token WHERE token_hash = ?;

-- name: ListTokens :many
SELECT * FROM token ORDER BY id DESC;

-- name: TouchToken :exec
UPDATE token SET last_used_at = current_timestamp WHERE token_hash = ?;

-- name: DeleteToken :execrows
DELETE FROM token WHERE id = ?;
