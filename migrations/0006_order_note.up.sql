-- ============================================================
-- Guest notes on orders ("no onions", allergy details). The guest
-- ordering page has always had a note box and the kitchen display
-- already renders order.note, but nothing was stored, so the note
-- never reached the kitchen. NOT NULL DEFAULT '' keeps every
-- existing row valid and avoids NULL handling in Go.
-- ============================================================
ALTER TABLE orders ADD COLUMN note TEXT NOT NULL DEFAULT '';
