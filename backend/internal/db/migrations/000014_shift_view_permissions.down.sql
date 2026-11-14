DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE code IN ('shifts.view', 'cash_movements.view'));

DELETE FROM permissions WHERE code IN ('shifts.view', 'cash_movements.view');
