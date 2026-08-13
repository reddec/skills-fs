-- name: CreateSkill :one
INSERT INTO skill (name, description, body, license, compatibility, allowed_tools, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetSkill :one
SELECT * FROM skill WHERE id = ?;

-- name: GetSkillByName :one
SELECT * FROM skill WHERE name = ?;

-- name: ListSkills :many
-- Summary without the (potentially large) body column.
SELECT id, name, description, license, compatibility, allowed_tools, metadata, created_at, updated_at
FROM skill
ORDER BY name;

-- name: UpdateSkill :exec
UPDATE skill SET
    name = ?,
    description = ?,
    body = ?,
    license = ?,
    compatibility = ?,
    allowed_tools = ?,
    metadata = ?,
    updated_at = current_timestamp
WHERE id = ?;

-- name: DeleteSkill :execrows
DELETE FROM skill WHERE id = ?;
