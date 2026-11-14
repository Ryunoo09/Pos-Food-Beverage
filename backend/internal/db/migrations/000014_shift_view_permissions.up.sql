-- Permission yang tertinggal dari seed awal: shifts.view dan cash_movements.view.
-- Tanpa ini endpoint ringkasan/laporan shift dan daftar kas tidak punya permission sah.

INSERT INTO permissions (code, module, description) VALUES
    ('shifts.view', 'shifts', 'Melihat ringkasan shift milik sendiri'),
    ('cash_movements.view', 'cash_movements', 'Melihat kas masuk/keluar')
ON CONFLICT (code) DO NOTHING;

-- Beri ke peran yang sudah memegang permission terkait (dev/DB lama).
INSERT INTO role_permissions (role_id, permission_id)
SELECT rp.role_id, p.id
FROM role_permissions rp
JOIN permissions src ON src.id = rp.permission_id AND src.code = 'shifts.open'
JOIN permissions p ON p.code = 'shifts.view'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT rp.role_id, p.id
FROM role_permissions rp
JOIN permissions src ON src.id = rp.permission_id AND src.code = 'payments.create'
JOIN permissions p ON p.code = 'cash_movements.view'
ON CONFLICT DO NOTHING;
