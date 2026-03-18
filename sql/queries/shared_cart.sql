-- name: CreateSharedCart :one
INSERT INTO shared_carts (
    id,
    table_session_id,
    created_by
)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCart :one
SELECT *
FROM shared_carts
WHERE id = $1;

-- name: AddCartParticipant :exec
INSERT INTO cart_participants (
    id,
    cart_id,
    customer_session_id
)
VALUES ($1, $2, $3);

-- name: GetCartParticipants :many
SELECT *
FROM cart_participants
WHERE cart_id = $1;