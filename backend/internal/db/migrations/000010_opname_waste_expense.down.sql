ALTER TABLE cash_movements DROP CONSTRAINT IF EXISTS fk_cash_movements_expense;

DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS expense_categories;
DROP TABLE IF EXISTS waste_records;
DROP TABLE IF EXISTS stock_opname_items;
DROP TABLE IF EXISTS stock_opnames;
