package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

// Parameter Argon2id. Cukup kuat untuk server kecil; ubah hanya bila profil
// keamanan berubah (hash lama tetap bisa diverifikasi karena parameter tersimpan).
const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID memvalidasi ID dari path sebelum dikirim ke query uuid, supaya input
// salah tidak menjadi error 500 dari PostgreSQL.
func IsUUID(s string) bool { return uuidPattern.MatchString(s) }

// Claims adalah isi access token. Permission tidak disimpan di token supaya
// perubahan hak akses langsung berlaku.
type Claims struct {
	BusinessID string `json:"business_id"`
	jwt.RegisteredClaims
}

// HashPassword menghasilkan hash Argon2id berformat standar:
// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hashPassword: baca salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword membandingkan password dengan hash tersimpan.
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}

	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ValidateUsername memastikan format username sesuai BR-F01-1.
func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"Username harus 3-32 karakter, hanya huruf kecil, angka, titik, garis bawah, atau strip.")
	}
	return nil
}

// ValidatePassword menerapkan BR-F01-7.
func ValidatePassword(password, username string) error {
	if len(password) < 10 || len(password) > 128 {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"Password harus 10-128 karakter.")
	}
	if strings.EqualFold(password, username) {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"Password tidak boleh sama dengan username.")
	}
	if weakPasswords[strings.ToLower(password)] {
		return httpx.NewError(http.StatusUnprocessableEntity, "VALIDATION_ERROR",
			"Password terlalu umum. Gunakan kombinasi yang lebih unik.")
	}
	return nil
}

// Daftar kecil password umum. Denylist penuh 1.000 entri ditunda (P1).
var weakPasswords = map[string]bool{
	"password123": true, "password1234": true, "1234567890": true, "qwerty12345": true,
	"admin12345": true, "admin123456": true, "letmein1234": true, "welcome1234": true,
	"iloveyou123": true, "password123!": true, "qwertyuiop": true, "12345678910": true,
	"rahasia1234": true, "indonesia123": true, "bismillah123": true, "password2024": true,
	"password2025": true, "password2026": true, "administrator": true, "changeme123": true,
}

// NewAccessToken menandatangani access token HS256.
func NewAccessToken(secret string, ttl time.Duration, userID, businessID string) (string, error) {
	now := time.Now()
	claims := Claims{
		BusinessID: businessID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("newAccessToken: tanda tangan: %w", err)
	}
	return signed, nil
}

// ParseAccessToken memverifikasi tanda tangan, algoritma, dan masa berlaku.
func ParseAccessToken(secret, raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// NewOpaqueToken membuat token acak dan hash SHA-256 untuk disimpan di DB.
func NewOpaqueToken() (raw string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("newOpaqueToken: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}

// HashToken menghitung hash token yang dikirim klien.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// NewUUID membuat UUID v4 tanpa dependency tambahan.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand gagal hanya bila sistem rusak; token tidak bisa dibuat.
		panic("auth: crypto/rand tidak tersedia")
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
