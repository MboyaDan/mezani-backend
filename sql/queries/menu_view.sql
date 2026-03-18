-- name: GetFullMenu :many
SELECT
    mc.id as category_id,
    mc.name as category_name,
    mc.display_order,
    COALESCE(
        json_agg(
            json_build_object(
                'id', mi.id,
                'name', mi.name,
                'description', mi.description,
                'price', mi.price,
                'available', mi.available,
                'sold_out', mi.sold_out,
                'is_special', mi.is_special
            ) ORDER BY mi.name
        ) FILTER (WHERE mi.id IS NOT NULL),
        '[]'::json
    )::json AS items
FROM menu_categories mc
LEFT JOIN menu_items mi ON mc.id = mi.category_id
    AND mi.available = TRUE
    AND mi.sold_out = FALSE
WHERE mc.menu_id = $1
GROUP BY mc.id, mc.name, mc.display_order
ORDER BY mc.display_order;