package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound  = errors.New("user tidak ditemukan")
	ErrTokenNotFound = errors.New("refresh token tidak ditemukan")
	ErrRoleNotFound  = errors.New("role tidak ditemukan")
)

// Repository menyimpan semua query F01. Tidak ada aturan bisnis di sini.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// FindUserByUsername mencari berdasarkan username (CITEXT = case-insensitive).
// MVP hanya mendukung satu bisnis; bila multi-bisnis aktif, login perlu
// parameter bisnis dan query ini harus difilter business_id.
func (r *Repository) FindUserByUsername(ctx context.Context, username string) (*User, error) {
	const q = `
		SELECT id, business_id, username, full_name, email, password_hash,
		       is_active, must_change_password, last_login_at
		FROM users
		WHERE username = $1
		ORDER BY created_at
		LIMIT 1`

	u := &User{}
	err := r.pool.QueryRow(ctx, q, username).Scan(
		&u.ID, &u.BusinessID, &u.Username, &u.FullName, &u.Email, &u.PasswordHash,
		&u.IsActive, &u.MustChangePassword, &u.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("findUserByUsername: %w", err)
	}
	return u, nil
}

func (r *Repository) FindUserByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, business_id, username, full_name, email, password_hash,
		       is_active, must_change_password, last_login_at
		FROM users
		WHERE id = $1`

	u := &User{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.BusinessID, &u.Username, &u.FullName, &u.Email, &u.PasswordHash,
		&u.IsActive, &u.MustChangePassword, &u.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("findUserByID: %w", err)
	}

	if u.Roles, err = r.RoleCodes(ctx, u.ID); err != nil {
		return nil, err
	}
	return u, nil
}

// DefaultOutletID mengembalikan outlet default bisnis. MVP satu outlet; bila
// multi-outlet aktif, outlet harus diambil dari konteks pengguna/pilihan UI.
func (r *Repository) DefaultOutletID(ctx context.Context, businessID string) (string, error) {
	const q = `
		SELECT id FROM outlets
		WHERE business_id = $1 AND is_active
		ORDER BY is_default DESC, created_at
		LIMIT 1`

	var id string
	err := r.pool.QueryRow(ctx, q, businessID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("defaultOutletID: %w", err)
	}
	return id, nil
}

func (r *Repository) RoleCodes(ctx context.Context, userID string) ([]string, error) {
	const q = `
		SELECT COALESCE(array_agg(r.code ORDER BY r.code), '{}')
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1`

	var codes []string
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&codes); err != nil {
		return nil, fmt.Errorf("roleCodes: %w", err)
	}
	return codes, nil
}

func (r *Repository) Permissions(ctx context.Context, userID string) ([]string, error) {
	const q = `
		SELECT COALESCE(array_agg(DISTINCT p.code ORDER BY p.code), '{}')
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1`

	var codes []string
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&codes); err != nil {
		return nil, fmt.Errorf("permissions: %w", err)
	}
	return codes, nil
}

func (r *Repository) CountRecentFailures(ctx context.Context, userID string, since time.Time) (int, error) {
	const q = `
		SELECT count(*) FROM login_histories
		WHERE user_id = $1 AND NOT success AND created_at > $2`

	var n int
	if err := r.pool.QueryRow(ctx, q, userID, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("countRecentFailures: %w", err)
	}
	return n, nil
}

func (r *Repository) InsertLoginHistory(ctx context.Context, h LoginHistory) error {
	const q = `
		INSERT INTO login_histories (user_id, username_attempted, success, failure_reason, ip_address, user_agent)
		VALUES (NULLIF($1, '')::uuid, $2, $3, NULLIF($4, ''), NULLIF($5, '')::inet, NULLIF($6, ''))`

	_, err := r.pool.Exec(ctx, q, h.UserID, h.UsernameAttempted, h.Success, h.FailureReason, h.IPAddress, h.UserAgent)
	if err != nil {
		return fmt.Errorf("insertLoginHistory: %w", err)
	}
	return nil
}

func (r *Repository) ListLoginHistories(ctx context.Context, businessID string, page, perPage int) ([]LoginHistory, int, error) {
	const q = `
		SELECT lh.id, COALESCE(lh.username_attempted, ''), lh.success,
		       COALESCE(lh.failure_reason, ''), COALESCE(lh.ip_address::text, ''),
		       COALESCE(lh.user_agent, ''), lh.created_at, COALESCE(u.full_name, '')
		FROM login_histories lh
		LEFT JOIN users u ON u.id = lh.user_id
		WHERE u.business_id = $1 OR lh.user_id IS NULL
		ORDER BY lh.created_at DESC
		LIMIT $2 OFFSET $3`

	const countQ = `
		SELECT count(*) FROM login_histories lh
		LEFT JOIN users u ON u.id = lh.user_id
		WHERE u.business_id = $1 OR lh.user_id IS NULL`

	var total int
	if err := r.pool.QueryRow(ctx, countQ, businessID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("listLoginHistories count: %w", err)
	}

	rows, err := r.pool.Query(ctx, q, businessID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listLoginHistories: %w", err)
	}
	defer rows.Close()

	items := []LoginHistory{}
	for rows.Next() {
		var h LoginHistory
		if err := rows.Scan(&h.ID, &h.UsernameAttempted, &h.Success, &h.FailureReason,
			&h.IPAddress, &h.UserAgent, &h.CreatedAt, &h.FullName); err != nil {
			return nil, 0, fmt.Errorf("scan login history: %w", err)
		}
		items = append(items, h)
	}
	return items, total, rows.Err()
}

func (r *Repository) InsertRefreshToken(ctx context.Context, tx pgx.Tx, t RefreshToken, hash []byte, ip, userAgent string) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::inet, NULLIF($7, ''))`

	_, err := tx.Exec(ctx, q, t.ID, t.UserID, t.FamilyID, hash, t.ExpiresAt, ip, userAgent)
	if err != nil {
		return fmt.Errorf("insertRefreshToken: %w", err)
	}
	return nil
}

func (r *Repository) FindRefreshToken(ctx context.Context, hash []byte) (*RefreshToken, error) {
	const q = `
		SELECT id, user_id, family_id, expires_at, used_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1`

	t := &RefreshToken{}
	err := r.pool.QueryRow(ctx, q, hash).Scan(&t.ID, &t.UserID, &t.FamilyID, &t.ExpiresAt, &t.UsedAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("findRefreshToken: %w", err)
	}
	return t, nil
}

// MarkTokenUsed mengembalikan false bila token sudah dipakai/dicabut (reuse).
func (r *Repository) MarkTokenUsed(ctx context.Context, tx pgx.Tx, id, replacedBy string) (bool, error) {
	const q = `
		UPDATE refresh_tokens SET used_at = now(), replaced_by = $2
		WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL`

	tag, err := tx.Exec(ctx, q, id, replacedBy)
	if err != nil {
		return false, fmt.Errorf("markTokenUsed: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) RevokeFamily(ctx context.Context, tx pgx.Tx, familyID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`

	if _, err := tx.Exec(ctx, q, familyID); err != nil {
		return fmt.Errorf("revokeFamily: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllTokens(ctx context.Context, tx pgx.Tx, userID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`

	if _, err := tx.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("revokeAllTokens: %w", err)
	}
	return nil
}

func (r *Repository) ListSessions(ctx context.Context, userID string, currentHash []byte) ([]Session, error) {
	const q = `
		SELECT id, COALESCE(ip_address::text, ''), COALESCE(user_agent, ''), issued_at, expires_at,
		       token_hash = $2 AS current
		FROM refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND used_at IS NULL AND expires_at > now()
		ORDER BY issued_at DESC`

	rows, err := r.pool.Query(ctx, q, userID, currentHash)
	if err != nil {
		return nil, fmt.Errorf("listSessions: %w", err)
	}
	defer rows.Close()

	items := []Session{}
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.IPAddress, &s.UserAgent, &s.IssuedAt, &s.ExpiresAt, &s.Current); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *Repository) SessionFamily(ctx context.Context, userID, sessionID string) (string, error) {
	const q = `SELECT family_id FROM refresh_tokens WHERE id = $1 AND user_id = $2`

	var familyID string
	err := r.pool.QueryRow(ctx, q, sessionID, userID).Scan(&familyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("sessionFamily: %w", err)
	}
	return familyID, nil
}

func (r *Repository) TouchLastLogin(ctx context.Context, userID string) error {
	const q = `UPDATE users SET last_login_at = now(), failed_login_count = 0 WHERE id = $1`

	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("touchLastLogin: %w", err)
	}
	return nil
}

func (r *Repository) ListUsers(ctx context.Context, businessID string, f UserFilter) ([]User, int, error) {
	const filter = `
		WHERE u.business_id = $1
		  AND ($2 = '' OR u.username ILIKE '%' || $2 || '%' OR u.full_name ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR EXISTS (
		      SELECT 1 FROM user_roles ur2 JOIN roles r2 ON r2.id = ur2.role_id
		      WHERE ur2.user_id = u.id AND r2.code = $3))
		  AND ($4::boolean IS NULL OR u.is_active = $4)`

	const listQ = `
		SELECT u.id, u.business_id, u.username, u.full_name, u.email,
		       u.is_active, u.must_change_password, u.last_login_at,
		       COALESCE(array_agg(DISTINCT r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		` + filter + `
		GROUP BY u.id
		ORDER BY u.created_at
		LIMIT $5 OFFSET $6`

	const countQ = `SELECT count(*) FROM users u ` + filter

	var total int
	if err := r.pool.QueryRow(ctx, countQ, businessID, f.Query, f.Role, f.IsActive).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("listUsers count: %w", err)
	}

	rows, err := r.pool.Query(ctx, listQ, businessID, f.Query, f.Role, f.IsActive, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listUsers: %w", err)
	}
	defer rows.Close()

	items := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.BusinessID, &u.Username, &u.FullName, &u.Email,
			&u.IsActive, &u.MustChangePassword, &u.LastLoginAt, &u.Roles); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		items = append(items, u)
	}
	return items, total, rows.Err()
}

func (r *Repository) CreateUser(ctx context.Context, tx pgx.Tx, u *User, roleIDs []string) error {
	const q = `
		INSERT INTO users (business_id, username, full_name, email, password_hash, must_change_password)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		RETURNING id`

	if err := tx.QueryRow(ctx, q, u.BusinessID, u.Username, u.FullName, nullable(u.Email), u.PasswordHash, u.MustChangePassword).Scan(&u.ID); err != nil {
		return fmt.Errorf("createUser: %w", err)
	}

	return r.replaceUserRoles(ctx, tx, u.ID, roleIDs)
}

func (r *Repository) UpdateUser(ctx context.Context, tx pgx.Tx, userID, fullName string) error {
	const q = `UPDATE users SET full_name = $2, updated_at = now() WHERE id = $1`

	if _, err := tx.Exec(ctx, q, userID, fullName); err != nil {
		return fmt.Errorf("updateUser: %w", err)
	}
	return nil
}

func (r *Repository) SetUserActive(ctx context.Context, tx pgx.Tx, userID string, active bool) error {
	const q = `UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`

	if _, err := tx.Exec(ctx, q, userID, active); err != nil {
		return fmt.Errorf("setUserActive: %w", err)
	}
	if !active {
		return r.RevokeAllTokens(ctx, tx, userID)
	}
	return nil
}

func (r *Repository) SetUserPassword(ctx context.Context, tx pgx.Tx, userID, hash string, mustChange bool) error {
	const q = `
		UPDATE users SET password_hash = $2, must_change_password = $3,
		       password_changed_at = now(), failed_login_count = 0, updated_at = now()
		WHERE id = $1`

	if _, err := tx.Exec(ctx, q, userID, hash, mustChange); err != nil {
		return fmt.Errorf("setUserPassword: %w", err)
	}
	return nil
}

func (r *Repository) ReplaceUserRoles(ctx context.Context, tx pgx.Tx, userID string, roleIDs []string) error {
	return r.replaceUserRoles(ctx, tx, userID, roleIDs)
}

func (r *Repository) replaceUserRoles(ctx context.Context, tx pgx.Tx, userID string, roleIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("replaceUserRoles delete: %w", err)
	}

	if len(roleIDs) == 0 {
		return nil
	}

	const q = `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, unnest($2::uuid[])`

	if _, err := tx.Exec(ctx, q, userID, roleIDs); err != nil {
		return fmt.Errorf("replaceUserRoles insert: %w", err)
	}
	return nil
}

func (r *Repository) RemoveUserRole(ctx context.Context, tx pgx.Tx, userID, roleID string) (bool, error) {
	const q = `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`

	tag, err := tx.Exec(ctx, q, userID, roleID)
	if err != nil {
		return false, fmt.Errorf("removeUserRole: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) CountActiveOwners(ctx context.Context, businessID string) (int, error) {
	const q = `
		SELECT count(DISTINCT u.id)
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE u.business_id = $1 AND u.is_active AND r.code = 'OWNER'`

	var n int
	if err := r.pool.QueryRow(ctx, q, businessID).Scan(&n); err != nil {
		return 0, fmt.Errorf("countActiveOwners: %w", err)
	}
	return n, nil
}

func (r *Repository) RolesByIDs(ctx context.Context, businessID string, ids []string) ([]Role, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	const q = `
		SELECT id, code, name, is_system FROM roles
		WHERE business_id = $1 AND id = ANY($2::uuid[])`

	rows, err := r.pool.Query(ctx, q, businessID, ids)
	if err != nil {
		return nil, fmt.Errorf("rolesByIDs: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.IsSystem); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *Repository) ListRoles(ctx context.Context, businessID string) ([]Role, error) {
	const q = `
		SELECT r.id, r.code, r.name, r.is_system,
		       COALESCE(array_agg(p.code ORDER BY p.code) FILTER (WHERE p.code IS NOT NULL), '{}')
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		WHERE r.business_id = $1
		GROUP BY r.id
		ORDER BY r.is_system DESC, r.code`

	rows, err := r.pool.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("listRoles: %w", err)
	}
	defer rows.Close()

	items := []Role{}
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.IsSystem, &role.Permissions); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		items = append(items, role)
	}
	return items, rows.Err()
}

func (r *Repository) ListPermissions(ctx context.Context) ([]Permission, error) {
	const q = `SELECT code, module, COALESCE(description, '') FROM permissions ORDER BY module, code`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("listPermissions: %w", err)
	}
	defer rows.Close()

	items := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Code, &p.Module, &p.Description); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// InsertAudit menulis audit log. Gagal menulis audit tidak membatalkan aksi
// utama, tetapi error dicatat oleh pemanggil.
func (r *Repository) InsertAudit(ctx context.Context, action, entityType, entityID string, a Actor, after any) error {
	const q = `
		INSERT INTO audit_logs (business_id, user_id, action, entity_type, entity_id,
		                        after_data, ip_address, user_agent, request_id)
		VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, ''), NULLIF($5, ''),
		        $6, NULLIF($7, '')::inet, NULLIF($8, ''), NULLIF($9, ''))`

	_, err := r.pool.Exec(ctx, q, a.BusinessID, a.UserID, action, entityType, entityID, after, a.IPAddress, a.UserAgent, a.RequestID)
	if err != nil {
		return fmt.Errorf("insertAudit: %w", err)
	}
	return nil
}

func nullable(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
