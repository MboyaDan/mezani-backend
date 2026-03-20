-- name: CreateActivity :exec
INSERT INTO staff_activities (
    id,
    staff_id,
    branch_id,
    action,
    entity_type,
    entity_id,
    old_data,
    new_data,
    ip_address,
    note
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);