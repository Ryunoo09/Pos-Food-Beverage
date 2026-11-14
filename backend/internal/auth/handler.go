package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

const refreshCookie = "refresh_token"

// Handler menerjemahkan HTTP request ke pemanggilan Service. Tidak ada aturan
// bisnis di sini.
type Handler struct {
	svc          *Service
	cookieSecure bool
	refreshTTL   time.Duration
}

func NewHandler(svc *Service, cookieSecure bool, refreshTTL time.Duration) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure, refreshTTL: refreshTTL}
}

// ---- auth ----

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	requestID := httpx.RequestIDFrom(r.Context())

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	result, err := h.svc.Login(r.Context(), body.Username, body.Password, metaFrom(r))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	h.setRefreshCookie(w, result.RefreshToken)
	httpx.WriteData(w, http.StatusOK, authPayload(result), requestID)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	requestID := httpx.RequestIDFrom(r.Context())

	result, err := h.svc.Refresh(r.Context(), h.refreshFrom(r), metaFrom(r))
	if err != nil {
		h.clearRefreshCookie(w)
		httpx.WriteError(w, err, requestID)
		return
	}

	h.setRefreshCookie(w, result.RefreshToken)
	httpx.WriteData(w, http.StatusOK, authPayload(result), requestID)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	requestID := httpx.RequestIDFrom(r.Context())

	if err := h.svc.Logout(r.Context(), h.refreshFrom(r)); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	if err := h.svc.LogoutAll(r.Context(), actor); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	user, perms, err := h.svc.Me(r.Context(), actor.UserID)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, map[string]any{
		"user":        user.Profile(),
		"permissions": perms,
	}, requestID)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	if err := h.svc.ChangePassword(r.Context(), actor, body.CurrentPassword, body.NewPassword); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	// Semua sesi dicabut oleh service; cookie lama tidak boleh dipakai lagi.
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	items, err := h.svc.Sessions(r.Context(), actor.UserID, h.refreshFrom(r))
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, items, requestID)
}

func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id := r.PathValue("id")
	if !IsUUID(id) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "SESSION_NOT_FOUND", "Sesi tidak ditemukan."), requestID)
		return
	}

	if err := h.svc.RevokeSession(r.Context(), actor.UserID, id); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LoginHistory(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	page, perPage := pagination(r)
	items, total, err := h.svc.LoginHistories(r.Context(), actor.BusinessID, page, perPage)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WritePage(w, items, pageMeta(page, perPage, total, requestID))
}

// ---- users ----

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	page, perPage := pagination(r)
	filter := UserFilter{
		Query:   r.URL.Query().Get("q"),
		Role:    r.URL.Query().Get("role"),
		Page:    page,
		PerPage: perPage,
	}
	if raw := r.URL.Query().Get("is_active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			httpx.WriteError(w, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
				"is_active harus true atau false."), requestID)
			return
		}
		filter.IsActive = &active
	}

	users, total, err := h.svc.ListUsers(r.Context(), actor.BusinessID, filter)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	profiles := make([]Profile, 0, len(users))
	for i := range users {
		profiles = append(profiles, users[i].Profile())
	}

	httpx.WritePage(w, profiles, pageMeta(page, perPage, total, requestID))
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	var body struct {
		Username string   `json:"username"`
		FullName string   `json:"full_name"`
		Email    *string  `json:"email"`
		Password string   `json:"password"`
		RoleIDs  []string `json:"role_ids"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	user, err := h.svc.CreateUser(r.Context(), actor, CreateUserInput{
		Username: body.Username,
		FullName: body.FullName,
		Email:    body.Email,
		Password: body.Password,
		RoleIDs:  body.RoleIDs,
	})
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusCreated, user.Profile(), requestID)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id := r.PathValue("id")
	if !IsUUID(id) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
		return
	}

	user, err := h.svc.GetUser(r.Context(), actor.BusinessID, id)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, user.Profile(), requestID)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id := r.PathValue("id")
	if !IsUUID(id) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
		return
	}

	var body struct {
		FullName string `json:"full_name"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	user, err := h.svc.UpdateUser(r.Context(), actor, id, body.FullName)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, user.Profile(), requestID)
}

func (h *Handler) SetUserActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, _ := ActorFrom(r.Context())
		requestID := httpx.RequestIDFrom(r.Context())

		id := r.PathValue("id")
		if !IsUUID(id) {
			httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
			return
		}

		if err := h.svc.SetUserActive(r.Context(), actor, id, active); err != nil {
			httpx.WriteError(w, err, requestID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id := r.PathValue("id")
	if !IsUUID(id) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
		return
	}

	var body struct {
		RoleIDs []string `json:"role_ids"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	if err := h.svc.SetUserRoles(r.Context(), actor, id, body.RoleIDs); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveUserRole(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id, roleID := r.PathValue("id"), r.PathValue("roleId")
	if !IsUUID(id) || !IsUUID(roleID) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
		return
	}

	if err := h.svc.RemoveUserRole(r.Context(), actor, id, roleID); err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	id := r.PathValue("id")
	if !IsUUID(id) {
		httpx.WriteError(w, httpx.NewError(http.StatusNotFound, "USER_NOT_FOUND", "Pengguna tidak ditemukan."), requestID)
		return
	}

	tempPassword, err := h.svc.ResetPassword(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, map[string]string{
		"temporary_password": tempPassword,
		"message":            "Sampaikan password sementara ke pengguna. Password wajib diganti saat login berikutnya.",
	}, requestID)
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	actor, _ := ActorFrom(r.Context())
	requestID := httpx.RequestIDFrom(r.Context())

	roles, err := h.svc.ListRoles(r.Context(), actor.BusinessID)
	if err != nil {
		httpx.WriteError(w, err, requestID)
		return
	}

	httpx.WriteData(w, http.StatusOK, roles, requestID)
}

// ---- helpers ----

func authPayload(result *AuthResult) map[string]any {
	return map[string]any{
		"access_token": result.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   result.ExpiresIn,
		"user":         result.Profile,
		"permissions":  result.Permissions,
	}
}

func metaFrom(r *http.Request) LoginMeta {
	return LoginMeta{
		IPAddress: httpx.ClientIP(r),
		UserAgent: truncate(r.UserAgent(), 500),
		RequestID: httpx.RequestIDFrom(r.Context()),
	}
}

func (h *Handler) refreshFrom(r *http.Request) string {
	cookie, err := r.Cookie(refreshCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

func pageMeta(page, perPage, total int, requestID string) httpx.PageMeta {
	totalPages := (total + perPage - 1) / perPage
	return httpx.PageMeta{
		Page: page, PerPage: perPage, Total: total,
		TotalPages: totalPages, RequestID: requestID,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
