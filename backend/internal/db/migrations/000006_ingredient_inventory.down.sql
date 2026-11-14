-- Lepas FK tertunda lebih dulu agar tabel ingredient bisa dihapus.
ALTER TABLE order_item_consumptions DROP CONSTRAINT IF EXISTS fk_order_item_consumptions_movement;
ALTER TABLE order_item_consumptions DROP CONSTRAINT IF EXISTS fk_order_item_consumptions_ingredient;
ALTER TABLE modifier_option_recipe_items DROP CONSTRAINT IF EXISTS fk_modifier_recipe_items_replaces;
ALTER TABLE modifier_option_recipe_items DROP CONSTRAINT IF EXISTS fk_modifier_recipe_items_ingredient;
ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_stock_ingredient;
ALTER TABLE products DROP CONSTRAINT IF EXISTS chk_products_stock_ingredient_required;

DROP TABLE IF EXISTS ingredient_cost_history;
DROP TRIGGER IF EXISTS trg_stock_movements_append_only ON stock_movements;
DROP FUNCTION IF EXISTS fn_deny_stock_movement_mutation();
DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS ingredient_unit_conversions;
DROP TABLE IF EXISTS ingredients;
DROP TABLE IF EXISTS ingredient_categories;
DROP TABLE IF EXISTS units;
