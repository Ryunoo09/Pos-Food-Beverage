-- Seed satuan dasar + konversi umum.
-- Role default (OWNER/MANAGER/CASHIER/KITCHEN_STAFF) TIDAK di-seed di SQL karena
-- roles.business_id NOT NULL dan bersifat per bisnis: dibuat aplikasi saat onboarding.
INSERT INTO units (id, code, name, dimension, to_base_factor, is_base) VALUES
    (1, 'g',     'gram',    'MASS',   1,      true),
    (2, 'kg',    'kilogram','MASS',   1000,   false),
    (3, 'ml',    'mililiter','VOLUME', 1,      true),
    (4, 'l',     'liter',   'VOLUME', 1000,   false),
    (5, 'pcs',   'pieces',  'COUNT',  1,      true),
    (6, 'lusin', 'lusin',   'COUNT',  12,     false)
ON CONFLICT (id) DO NOTHING;
