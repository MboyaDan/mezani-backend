-- name: CreateTableSession :one
INSERT INTO table_sessions (
    id,
    table_id,
    status,
    expires_at
)
VALUES ($1, $2, 'active', $3)
RETURNING *;


-- name: GetTableSession :one
SELECT *
FROM table_sessions
WHERE id = $1;


-- name: GetActiveTableSessionByTable :one
SELECT *
FROM table_sessions
WHERE table_id = $1
AND status = 'active'
LIMIT 1;


-- name: CloseTableSession :exec
UPDATE table_sessions
SET status = 'closed'
WHERE id = $1;


-- name: GetExpiredSessions :many
SELECT *
FROM table_sessions
WHERE status = 'active'
AND expires_at < NOW();

-- name: ExtendTableSession :exec
UPDATE table_sessions
SET expires_at = NOW() + INTERVAL '30 minutes'
WHERE id = $1
AND status = 'active';

-- name: GetTableSessionWithTable :one
SELECT 
    ts.id,
    ts.table_id,
    ts.status,
    ts.expires_at,
    ts.created_at,
    t.branch_id,
    t.table_number  
FROM table_sessions ts
JOIN tables t ON t.id = ts.table_id
WHERE ts.id = $1;