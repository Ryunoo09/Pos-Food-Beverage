package auth

import (
	"testing"
	"time"
)

func TestHashPassword_VerifyRoundTrip(t *testing.T) {	hash, err := HashPassword("rahasia-yang-kuat")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword("rahasia-yang-kuat", hash) {
		t.Error("password benar ditolak")
	}
	if VerifyPassword("rahasia-yang-salah", hash) {
		t.Error("password salah diterima")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2id$v=19$badsalt$", "$bcrypt$v=19$m=1,t=1,p=1$a$b"} {
		if VerifyPassword("apa-saja", h) {
			t.Errorf("hash %q diterima", h)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	valid := []string{"kasir1", "budi.santoso", "dewi_01", "a-b-c"}
	for _, name := range valid {
		if err := ValidateUsername(name); err != nil {
			t.Errorf("%q ditolak: %v", name, err)
		}
	}

	invalid := []string{"ab", "Budi", "budi santoso", "budi@mail", ""}
	for _, name := range invalid {
		if err := ValidateUsername(name); err == nil {
			t.Errorf("%q diterima", name)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("pendek", "kasir1"); err == nil {
		t.Error("password pendek diterima")
	}
	if err := ValidatePassword("kasir1", "kasir1"); err == nil {
		t.Error("password sama dengan username diterima")
	}
	if err := ValidatePassword("password123", "kasir1"); err == nil {
		t.Error("password denylist diterima")
	}
	if err := ValidatePassword("kopi-susu-enak", "kasir1"); err != nil {
		t.Errorf("password valid ditolak: %v", err)
	}
}

func TestAccessToken_RoundTrip(t *testing.T) {
	raw, err := NewAccessToken("secret", time.Minute, "user-1", "biz-1")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := ParseAccessToken("secret", raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != "user-1" || claims.BusinessID != "biz-1" {
		t.Errorf("claims = %+v", claims)
	}

	if _, err := ParseAccessToken("secret-lain", raw); err == nil {
		t.Error("token dengan secret lain diterima")
	}
}

func TestAccessToken_Expired(t *testing.T) {
	raw, err := NewAccessToken("secret", -time.Minute, "user-1", "biz-1")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ParseAccessToken("secret", raw); err == nil {
		t.Error("token kedaluwarsa diterima")
	}
}

func TestOpaqueToken_HashMatches(t *testing.T) {
	raw, hash, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if raw == "" || len(hash) != 32 {
		t.Fatalf("raw=%q len(hash)=%d", raw, len(hash))
	}
	if got := HashToken(raw); string(got) != string(hash) {
		t.Error("HashToken tidak sama dengan hash saat dibuat")
	}
}

func TestIsUUID(t *testing.T) {
	if !IsUUID("3f2504e0-4f89-41d3-9a0c-0305e82c3301") {
		t.Error("uuid valid ditolak")
	}
	for _, bad := range []string{"", "abc", "3f2504e0-4f89-41d3-9a0c", "'; DROP TABLE users;--"} {
		if IsUUID(bad) {
			t.Errorf("%q diterima sebagai uuid", bad)
		}
	}
}

func TestGuardRoleAssignment(t *testing.T) {
	svc := &Service{}
	manager := Actor{Roles: []string{"MANAGER"}}
	owner := Actor{Roles: []string{"OWNER"}}
	ownerRole := []Role{{Code: "OWNER"}}
	managerRole := []Role{{Code: "MANAGER"}}
	cashierRole := []Role{{Code: "CASHIER"}}

	if err := svc.guardRoleAssignment(manager, ownerRole, nil); err == nil {
		t.Error("MANAGER bisa memberi role OWNER")
	}
	if err := svc.guardRoleAssignment(manager, nil, []string{"MANAGER"}); err == nil {
		t.Error("MANAGER bisa mencabut role MANAGER")
	}
	if err := svc.guardRoleAssignment(manager, cashierRole, nil); err != nil {
		t.Errorf("MANAGER ditolak saat memberi CASHIER: %v", err)
	}
	if err := svc.guardRoleAssignment(owner, managerRole, nil); err != nil {
		t.Errorf("OWNER ditolak saat memberi MANAGER: %v", err)
	}
	if err := svc.guardRoleAssignment(owner, ownerRole, []string{"OWNER"}); err != nil {
		t.Errorf("OWNER ditolak saat menyentuh OWNER: %v", err)
	}
}

// TestNewSSEToken_ShortLivedWithScope: token SSE membawa scope "sse" dan
// kedaluwarsa 60 detik, sehingga tidak bisa dipakai sebagai access token biasa.
func TestNewSSEToken_ShortLivedWithScope(t *testing.T) {
	const secret = "rahasia-test"
	token, err := NewSSEToken(secret, 60*time.Second, "user-1", "biz-1")
	if err != nil {
		t.Fatalf("buat token: %v", err)
	}

	claims, err := ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Scope != "sse" {
		t.Errorf("scope = %q, mau \"sse\"", claims.Scope)
	}
	if claims.Subject != "user-1" || claims.BusinessID != "biz-1" {
		t.Errorf("claims salah: %+v", claims)
	}
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) > 61*time.Second {
		t.Errorf("umur token tidak ≤ 60 detik: %v", claims.ExpiresAt)
	}
}
