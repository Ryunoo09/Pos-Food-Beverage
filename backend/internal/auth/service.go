package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

const (
	lockoutWindow = 15 * time.Minute
	maxFailures   = 5
)

// Service memuat seluruh aturan bisnis F01.
type Service struct {
	repo       *Repository
	secret     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewService(repo *Repository, secret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       repo,
		secret:     secret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// dummyHash dipakai agar login dengan username tidak dikenal tetap menghitung
// Argon2 (waktu respons mirip) sehingga keberadaan username tidak bocor.
var dummyHash = sync.OnceValue(func() string {
	h, err := HashPassword("dummy-password-for-timing")
	if err != nil {
		return ""
	}
	return h
})

// Login memvalidasi kredensial, menegakkan lockout, lalu menerbitkan token.
func (s *Service) Login(ctx context.Context, username, password string, meta LoginMeta) (*AuthResult, error) {
	username = strings.ToLower(strings.TrimSpace(username))

	if ValidateUsername(username) != nil || password == "" || len(password) > 128 {
		return nil, invalidCredentials()
	}

	user, err := s.repo.FindUserByUsername(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		VerifyPassword(password, dummyHash())
		s.recordLogin(ctx, "", username, false, "user_not_found", meta)
		return nil, invalidCredentials()
	}
	if err != nil {
		return nil, err
	}

	failures, err := s.repo.CountRecentFailures(ctx, user.ID, s.now().Add(-lockoutWindow))
	if err != nil {
		return nil, err
	}
	if failures >= maxFailures {
		s.recordLogin(ctx, user.ID, username, false, "locked", meta)
		return nil, &httpx.AppError{
			Code:       "AUTH_ACCOUNT_LOCKED",
			Message:    "Akun terkunci sementara karena terlalu banyak percobaan login. Coba lagi nanti.",
			Status:     http.StatusTooManyRequests,
			RetryAfter: int(lockoutWindow.Seconds()),
		}
	}

	if !VerifyPassword(password, user.PasswordHash) {
		s.recordLogin(ctx, user.ID, username, false, "invalid_password", meta)
		return nil, invalidCredentials()
	}

	if !user.IsActive {
		s.recordLogin(ctx, user.ID, username, false, "inactive", meta)
		return nil, httpx.NewError(http.StatusUnauthorized, "AUTH_ACCOUNT_INACTIVE",
			"Akun tidak aktif. Hubungi pemilik usaha.")
	}

	user.Roles, err = s.repo.RoleCodes(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	perms, err := s.repo.Permissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}

	s.recordLogin(ctx, user.ID, username, true, "", meta)
	if err := s.repo.TouchLastLogin(ctx, user.ID); err != nil {
		slog.Warn("gagal memperbarui last_login_at", "user_id", user.ID, "error", err)
	}
	s.audit(ctx, "auth.login", "user", user.ID, Actor{
		UserID: user.ID, BusinessID: user.BusinessID, RequestID: meta.RequestID,
		IPAddress: meta.IPAddress, UserAgent: meta.UserAgent,
	}, nil)

	return &AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: refreshToken,
		Profile:      user.Profile(),
		Permissions:  perms,
	}, nil
}

// Refresh merotasi refresh token. Token yang sudah dipakai dianggap reuse dan
// seluruh keluarga token dicabut.
func (s *Service) Refresh(ctx context.Context, rawToken string, meta LoginMeta) (*AuthResult, error) {
	if rawToken == "" {
		return nil, refreshInvalid()
	}

	token, err := s.repo.FindRefreshToken(ctx, HashToken(rawToken))
	if errors.Is(err, ErrTokenNotFound) {
		return nil, refreshInvalid()
	}
	if err != nil {
		return nil, err
	}

	if token.UsedAt != nil {
		s.revokeFamily(ctx, token.FamilyID)
		return nil, httpx.NewError(http.StatusUnauthorized, "AUTH_REFRESH_REUSED",
			"Sesi tidak valid karena token sudah dipakai. Silakan login ulang.")
	}
	if token.RevokedAt != nil || !token.ExpiresAt.After(s.now()) {
		return nil, refreshInvalid()
	}

	user, err := s.repo.FindUserByID(ctx, token.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		s.revokeFamily(ctx, token.FamilyID)
		return nil, httpx.NewError(http.StatusUnauthorized, "AUTH_ACCOUNT_INACTIVE",
			"Akun tidak aktif. Hubungi pemilik usaha.")
	}

	raw, hash, err := NewOpaqueToken()
	if err != nil {
		return nil, err
	}
	newID := NewUUID()

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("refresh: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert token baru dulu: kolom replaced_by punya FK ke refresh_tokens.id.
	if err := s.repo.InsertRefreshToken(ctx, tx, RefreshToken{
		ID:        newID,
		UserID:    user.ID,
		FamilyID:  token.FamilyID,
		ExpiresAt: s.now().Add(s.refreshTTL),
	}, hash, meta.IPAddress, meta.UserAgent); err != nil {
		return nil, err
	}

	rotated, err := s.repo.MarkTokenUsed(ctx, tx, token.ID, newID)
	if err != nil {
		return nil, err
	}
	if !rotated {
		// Balapan: token dipakai request lain di antara SELECT dan UPDATE.
		tx.Rollback(ctx)
		s.revokeFamily(ctx, token.FamilyID)
		return nil, httpx.NewError(http.StatusUnauthorized, "AUTH_REFRESH_REUSED",
			"Sesi tidak valid karena token sudah dipakai. Silakan login ulang.")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("refresh: commit: %w", err)
	}

	accessToken, err := NewAccessToken(s.secret, s.accessTTL, user.ID, user.BusinessID)
	if err != nil {
		return nil, err
	}
	perms, err := s.repo.Permissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken:  accessToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: raw,
		Profile:      user.Profile(),
		Permissions:  perms,
	}, nil
}

// Logout mencabut keluarga token milik sesi saat ini.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}

	token, err := s.repo.FindRefreshToken(ctx, HashToken(rawToken))
	if errors.Is(err, ErrTokenNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("logout: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.RevokeFamily(ctx, tx, token.FamilyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) LogoutAll(ctx context.Context, actor Actor) error {
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("logoutAll: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.RevokeAllTokens(ctx, tx, actor.UserID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.audit(ctx, "auth.logout_all", "user", actor.UserID, actor, nil)
	return nil
}

func (s *Service) Me(ctx context.Context, userID string) (*User, []string, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	perms, err := s.repo.Permissions(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return user, perms, nil
}

// sseTokenTTL singkat: token muncul di URL (EventSource tanpa header custom).
const sseTokenTTL = 60 * time.Second

// SSEToken menerbitkan token khusus koneksi SSE; tidak berlaku untuk endpoint
// lain karena middleware AuthRequired menolak scope "sse".
func (s *Service) SSEToken(userID, businessID string) (string, int, error) {
	token, err := NewSSEToken(s.secret, sseTokenTTL, userID, businessID)
	if err != nil {
		return "", 0, err
	}
	return token, int(sseTokenTTL.Seconds()), nil
}

// ChangePassword mewajibkan password lama dan mencabut seluruh sesi.
func (s *Service) ChangePassword(ctx context.Context, actor Actor, current, next string) error {
	user, err := s.repo.FindUserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !VerifyPassword(current, user.PasswordHash) {
		return httpx.NewError(http.StatusBadRequest, "AUTH_INVALID_CREDENTIALS",
			"Password lama salah.")
	}
	if err := ValidatePassword(next, user.Username); err != nil {
		return err
	}

	hash, err := HashPassword(next)
	if err != nil {
		return err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("changePassword: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetUserPassword(ctx, tx, user.ID, hash, false); err != nil {
		return err
	}
	if err := s.repo.RevokeAllTokens(ctx, tx, user.ID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.audit(ctx, "auth.change_password", "user", user.ID, actor, nil)
	return nil
}

func (s *Service) Sessions(ctx context.Context, userID, currentToken string) ([]Session, error) {
	var hash []byte
	if currentToken != "" {
		hash = HashToken(currentToken)
	}

	items, err := s.repo.ListSessions(ctx, userID, hash)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// RevokeSession mencabut satu sesi (seluruh keluarga token tersebut).
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	familyID, err := s.repo.SessionFamily(ctx, userID, sessionID)
	if errors.Is(err, ErrTokenNotFound) {
		return httpx.NewError(http.StatusNotFound, "SESSION_NOT_FOUND", "Sesi tidak ditemukan.")
	}
	if err != nil {
		return err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("revokeSession: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.RevokeFamily(ctx, tx, familyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LoadActor mengambil role aktual dari DB (bukan dari token) agar perubahan
// hak akses langsung berlaku.
func (s *Service) LoadActor(ctx context.Context, userID, businessID, requestID, ip, userAgent string) (Actor, error) {
	roles, err := s.repo.RoleCodes(ctx, userID)
	if err != nil {
		return Actor{}, err
	}
	outletID, err := s.repo.DefaultOutletID(ctx, businessID)
	if err != nil {
		return Actor{}, err
	}
	return Actor{
		UserID: userID, BusinessID: businessID, OutletID: outletID, Roles: roles,
		RequestID: requestID, IPAddress: ip, UserAgent: userAgent,
	}, nil
}

func (s *Service) Permissions(ctx context.Context, userID string) ([]string, error) {
	return s.repo.Permissions(ctx, userID)
}

func (s *Service) ListUsers(ctx context.Context, businessID string, f UserFilter) ([]User, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 20
	}
	return s.repo.ListUsers(ctx, businessID, f)
}

func (s *Service) GetUser(ctx context.Context, businessID, id string) (*User, error) {
	user, err := s.repo.FindUserByID(ctx, id)
	if errors.Is(err, ErrUserNotFound) {
		return nil, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan.")
	}
	if err != nil {
		return nil, err
	}
	if user.BusinessID != businessID {
		return nil, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan.")
	}
	return user, nil
}

type CreateUserInput struct {
	Username string
	FullName string
	Email    *string
	Password string
	RoleIDs  []string
}

func (s *Service) CreateUser(ctx context.Context, actor Actor, in CreateUserInput) (*User, error) {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.FullName = strings.TrimSpace(in.FullName)

	if err := ValidateUsername(in.Username); err != nil {
		return nil, err
	}
	if in.FullName == "" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Nama lengkap wajib diisi.")
	}
	if err := ValidatePassword(in.Password, in.Username); err != nil {
		return nil, err
	}
	if len(in.RoleIDs) == 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Minimal satu role harus dipilih.")
	}

	roles, err := s.repo.RolesByIDs(ctx, actor.BusinessID, in.RoleIDs)
	if err != nil {
		return nil, err
	}
	if len(roles) != len(in.RoleIDs) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Role tidak valid untuk bisnis ini.")
	}
	if err := s.guardRoleAssignment(actor, roles, nil); err != nil {
		return nil, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	user := &User{
		BusinessID:   actor.BusinessID,
		Username:     in.Username,
		FullName:     in.FullName,
		Email:        in.Email,
		PasswordHash: hash,
		IsActive:     true,
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createUser: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateUser(ctx, tx, user, in.RoleIDs); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, httpx.NewError(http.StatusConflict, "USERNAME_TAKEN", "Username sudah dipakai.")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createUser: commit: %w", err)
	}

	user.Roles = roleCodes(roles)
	s.audit(ctx, "user.create", "user", user.ID, actor, user.Profile())
	return user, nil
}

func (s *Service) UpdateUser(ctx context.Context, actor Actor, id, fullName string) (*User, error) {
	user, err := s.GetUser(ctx, actor.BusinessID, id)
	if err != nil {
		return nil, err
	}

	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Nama lengkap wajib diisi.")
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("updateUser: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.UpdateUser(ctx, tx, id, fullName); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("updateUser: commit: %w", err)
	}

	user.FullName = fullName
	s.audit(ctx, "user.update", "user", id, actor, user.Profile())
	return user, nil
}

func (s *Service) SetUserActive(ctx context.Context, actor Actor, id string, active bool) error {
	user, err := s.GetUser(ctx, actor.BusinessID, id)
	if err != nil {
		return err
	}

	if !active {
		if id == actor.UserID {
			return httpx.NewError(http.StatusUnprocessableEntity, "USER_SELF_DEACTIVATE",
				"Anda tidak dapat menonaktifkan akun sendiri.")
		}
		if contains(user.Roles, "OWNER") {
			count, err := s.repo.CountActiveOwners(ctx, actor.BusinessID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return httpx.NewError(http.StatusUnprocessableEntity, "USER_LAST_OWNER",
					"Sistem harus memiliki minimal satu OWNER aktif.")
			}
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("setUserActive: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetUserActive(ctx, tx, id, active); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("setUserActive: commit: %w", err)
	}

	action := "user.activate"
	if !active {
		action = "user.deactivate"
	}
	s.audit(ctx, action, "user", id, actor, nil)
	return nil
}

// SetUserRoles menetapkan set role penuh dengan guard eskalasi (BR-F01-4/5).
func (s *Service) SetUserRoles(ctx context.Context, actor Actor, id string, roleIDs []string) error {
	if id == actor.UserID {
		return httpx.NewError(http.StatusUnprocessableEntity, "USER_SELF_ROLE_CHANGE",
			"Anda tidak dapat mengubah role sendiri.")
	}
	if len(roleIDs) == 0 {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Minimal satu role harus dipilih.")
	}

	user, err := s.GetUser(ctx, actor.BusinessID, id)
	if err != nil {
		return err
	}

	roles, err := s.repo.RolesByIDs(ctx, actor.BusinessID, roleIDs)
	if err != nil {
		return err
	}
	if len(roles) != len(roleIDs) {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Role tidak valid untuk bisnis ini.")
	}

	if err := s.guardRoleAssignment(actor, roles, user.Roles); err != nil {
		return err
	}

	if contains(user.Roles, "OWNER") && !contains(roleCodes(roles), "OWNER") && user.IsActive {
		count, err := s.repo.CountActiveOwners(ctx, actor.BusinessID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return httpx.NewError(http.StatusUnprocessableEntity, "USER_LAST_OWNER",
				"Sistem harus memiliki minimal satu OWNER aktif.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("setUserRoles: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.ReplaceUserRoles(ctx, tx, id, roleIDs); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("setUserRoles: commit: %w", err)
	}

	s.audit(ctx, "user.set_roles", "user", id, actor, roleCodes(roles))
	return nil
}

// RemoveUserRole mencabut satu role dari pengguna dengan guard eskalasi dan
// aturan minimal satu OWNER aktif.
func (s *Service) RemoveUserRole(ctx context.Context, actor Actor, userID, roleID string) error {
	if userID == actor.UserID {
		return httpx.NewError(http.StatusUnprocessableEntity, "USER_SELF_ROLE_CHANGE",
			"Anda tidak dapat mengubah role sendiri.")
	}

	user, err := s.GetUser(ctx, actor.BusinessID, userID)
	if err != nil {
		return err
	}

	roles, err := s.repo.RolesByIDs(ctx, actor.BusinessID, []string{roleID})
	if err != nil {
		return err
	}
	if len(roles) != 1 {
		return httpx.NewError(http.StatusNotFound, "ROLE_NOT_FOUND", "Role tidak ditemukan.")
	}
	target := roles[0]

	if err := s.guardRoleAssignment(actor, nil, []string{target.Code}); err != nil {
		return err
	}
	if len(user.Roles) <= 1 {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"Pengguna harus memiliki minimal satu role.")
	}
	if target.Code == "OWNER" && user.IsActive {
		count, err := s.repo.CountActiveOwners(ctx, actor.BusinessID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return httpx.NewError(http.StatusUnprocessableEntity, "USER_LAST_OWNER",
				"Sistem harus memiliki minimal satu OWNER aktif.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return fmt.Errorf("removeUserRole: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	removed, err := s.repo.RemoveUserRole(ctx, tx, userID, roleID)
	if err != nil {
		return err
	}
	if !removed {
		return httpx.NewError(http.StatusNotFound, "ROLE_NOT_ASSIGNED", "Role tersebut tidak dimiliki pengguna ini.")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("removeUserRole: commit: %w", err)
	}

	s.audit(ctx, "user.remove_role", "user", userID, actor, target.Code)
	return nil
}

// ResetPassword menghasilkan password sementara sekali pakai.
func (s *Service) ResetPassword(ctx context.Context, actor Actor, id string) (string, error) {
	user, err := s.GetUser(ctx, actor.BusinessID, id)
	if err != nil {
		return "", err
	}
	if id == actor.UserID {
		return "", httpx.NewError(http.StatusUnprocessableEntity, "USER_SELF_RESET",
			"Gunakan Ganti Password untuk akun sendiri.")
	}

	tempPassword, err := temporaryPassword(12)
	if err != nil {
		return "", err
	}
	hash, err := HashPassword(tempPassword)
	if err != nil {
		return "", err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("resetPassword: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetUserPassword(ctx, tx, user.ID, hash, true); err != nil {
		return "", err
	}
	if err := s.repo.RevokeAllTokens(ctx, tx, user.ID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("resetPassword: commit: %w", err)
	}

	s.audit(ctx, "user.reset_password", "user", user.ID, actor, nil)
	return tempPassword, nil
}

func (s *Service) ListRoles(ctx context.Context, businessID string) ([]Role, error) {
	return s.repo.ListRoles(ctx, businessID)
}

func (s *Service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) LoginHistories(ctx context.Context, businessID string, page, perPage int) ([]LoginHistory, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return s.repo.ListLoginHistories(ctx, businessID, page, perPage)
}

func (s *Service) issueTokens(ctx context.Context, user *User, meta LoginMeta) (string, string, error) {
	accessToken, err := NewAccessToken(s.secret, s.accessTTL, user.ID, user.BusinessID)
	if err != nil {
		return "", "", err
	}

	raw, hash, err := NewOpaqueToken()
	if err != nil {
		return "", "", err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return "", "", fmt.Errorf("issueTokens: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.InsertRefreshToken(ctx, tx, RefreshToken{
		ID:        NewUUID(),
		UserID:    user.ID,
		FamilyID:  NewUUID(),
		ExpiresAt: s.now().Add(s.refreshTTL),
	}, hash, meta.IPAddress, meta.UserAgent); err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("issueTokens: commit: %w", err)
	}

	return accessToken, raw, nil
}

// guardRoleAssignment menolak pemberian/penghapusan role di atas level aktor.
// Hanya OWNER yang boleh menyentuh OWNER dan MANAGER (BR-F01-4).
func (s *Service) guardRoleAssignment(actor Actor, next []Role, current []string) error {
	if contains(actor.Roles, "OWNER") {
		return nil
	}

	for _, role := range next {
		if role.Code == "OWNER" || role.Code == "MANAGER" {
			return forbiddenRole()
		}
	}
	for _, code := range current {
		if code == "OWNER" || code == "MANAGER" {
			return forbiddenRole()
		}
	}
	return nil
}

func (s *Service) recordLogin(ctx context.Context, userID, username string, success bool, reason string, meta LoginMeta) {
	if err := s.repo.InsertLoginHistory(ctx, LoginHistory{
		UserID:            userID,
		UsernameAttempted: username,
		Success:           success,
		FailureReason:     reason,
		IPAddress:         meta.IPAddress,
		UserAgent:         meta.UserAgent,
	}); err != nil {
		slog.Warn("gagal mencatat login history", "error", err)
	}
}

func (s *Service) revokeFamily(ctx context.Context, familyID string) {
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		slog.Error("gagal mencabut keluarga token", "family_id", familyID, "error", err)
		return
	}
	defer tx.Rollback(ctx)

	if err := s.repo.RevokeFamily(ctx, tx, familyID); err != nil {
		slog.Error("gagal mencabut keluarga token", "family_id", familyID, "error", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Error("gagal commit pencabutan token", "family_id", familyID, "error", err)
	}
}

func (s *Service) audit(ctx context.Context, action, entityType, entityID string, actor Actor, after any) {
	var payload json.RawMessage
	if after != nil {
		if raw, err := json.Marshal(after); err == nil {
			payload = raw
		}
	}

	if err := s.repo.InsertAudit(ctx, action, entityType, entityID, actor, payload); err != nil {
		slog.Warn("gagal menulis audit log", "action", action, "error", err)
	}
}

func invalidCredentials() *httpx.AppError {
	return httpx.NewError(http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS",
		"Username atau password salah.")
}

func refreshInvalid() *httpx.AppError {
	return httpx.NewError(http.StatusUnauthorized, "AUTH_TOKEN_EXPIRED",
		"Sesi sudah berakhir. Silakan login ulang.")
}

func forbiddenRole() *httpx.AppError {
	return httpx.NewError(http.StatusForbidden, "FORBIDDEN_ROLE_ASSIGNMENT",
		"Anda tidak berwenang memberikan atau mencabut role tersebut.")
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func roleCodes(roles []Role) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.Code)
	}
	return codes
}

// temporaryPassword menghasilkan password acak yang memenuhi panjang minimum.
func temporaryPassword(length int) (string, error) {
	const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("temporaryPassword: %w", err)
		}
		out[i] = charset[n.Int64()]
	}
	return string(out), nil
}
