package auth

import (
	"time"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/audit"
)

// User adalah baris minimal users yang dibutuhkan autentikasi.
type User struct {
	ID                 string
	BusinessID         string
	Username           string
	FullName           string
	Email              *string
	PasswordHash       string
	IsActive           bool
	MustChangePassword bool
	LastLoginAt        *time.Time
	Roles              []string
}

// RefreshToken menyimpan hash token, bukan token mentah.
type RefreshToken struct {
	ID        string
	UserID    string
	FamilyID  string
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

// LoginHistory adalah satu percobaan login.
type LoginHistory struct {
	ID                string    `json:"id"`
	UserID            string    `json:"-"`
	UsernameAttempted string    `json:"username_attempted"`
	Success           bool      `json:"success"`
	FailureReason     string    `json:"failure_reason,omitempty"`
	IPAddress         string    `json:"ip_address,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	FullName          string    `json:"full_name,omitempty"`
}

// Session adalah refresh token aktif yang ditampilkan ke pengguna.
type Session struct {
	ID        string    `json:"id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Current   bool      `json:"current"`
}

type Permission struct {
	Code        string `json:"code"`
	Module      string `json:"module"`
	Description string `json:"description,omitempty"`
}

type Role struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions,omitempty"`
}

// Profile adalah data pengguna yang aman dikirim ke klien.
type Profile struct {
	ID                 string   `json:"id"`
	Username           string   `json:"username"`
	FullName           string   `json:"full_name"`
	Email              *string  `json:"email,omitempty"`
	IsActive           bool     `json:"is_active"`
	MustChangePassword bool     `json:"must_change_password"`
	Roles              []string `json:"roles"`
}

func (u *User) Profile() Profile {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return Profile{
		ID:                 u.ID,
		Username:           u.Username,
		FullName:           u.FullName,
		Email:              u.Email,
		IsActive:           u.IsActive,
		MustChangePassword: u.MustChangePassword,
		Roles:              roles,
	}
}

// AuthResult dikembalikan saat login/refresh. RefreshToken tidak masuk body,
// hanya dipasang sebagai cookie oleh handler.
type AuthResult struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	Profile      Profile
	Permissions  []string
}

// Actor adalah pengguna yang sedang melakukan aksi, dipakai untuk guard role.
type Actor struct {
	UserID      string
	BusinessID  string
	OutletID    string
	Roles       []string
	Permissions []string
	RequestID   string
	IPAddress   string
	UserAgent   string
}

// Audit mengisi konteks penulis audit dari aktor terautentikasi.
func (a Actor) Audit() audit.Actor {
	return audit.Actor{
		BusinessID: a.BusinessID,
		OutletID:   a.OutletID,
		UserID:     a.UserID,
		Roles:      a.Roles,
		IPAddress:  a.IPAddress,
		UserAgent:  a.UserAgent,
		RequestID:  a.RequestID,
	}
}

type LoginMeta struct {
	IPAddress string
	UserAgent string
	RequestID string
}

type UserFilter struct {
	Query    string
	Role     string
	IsActive *bool
	Page     int
	PerPage  int
}
