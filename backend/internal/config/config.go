package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config hanya memuat nilai yang sudah dipakai. Nilai lain (backup, payment)
// ditambahkan saat fiturnya masuk, supaya tidak ada config mati.
type Config struct {
	Env             string
	Port            string
	DatabaseURL     string
	ShutdownTimeout time.Duration

	JWTSecret    string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool

	// Backup F21. BackupDir kosong = backup terjadwal dimatikan (dev).
	BackupDir             string
	BackupEncryptionKey   string
	BackupRetentionDays   int
	BackupScheduleHour    int
}

func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		Env:             env("APP_ENV", "development"),
		Port:            env("PORT", "8080"),
		DatabaseURL:     env("DATABASE_URL", ""),
		ShutdownTimeout: 10 * time.Second,
		JWTSecret:       env("JWT_SECRET", "dev-insecure-jwt-secret-change-me"),
		CookieSecure:    env("COOKIE_SECURE", "false") == "true",

		BackupDir:           env("BACKUP_DIR", ""),
		BackupEncryptionKey: env("BACKUP_ENCRYPTION_KEY", ""),
		BackupRetentionDays: envInt("BACKUP_RETENTION_DAILY", 14),
		BackupScheduleHour:  envInt("BACKUP_SCHEDULE_HOUR", 2),
	}

	var err error
	if cfg.AccessTTL, err = envDuration("JWT_ACCESS_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RefreshTTL, err = envDuration("JWT_REFRESH_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}

	// Produksi tidak boleh memakai secret default: token bisa dipalsukan.
	if cfg.Env == "production" && cfg.JWTSecret == "dev-insecure-jwt-secret-change-me" {
		return Config{}, fmt.Errorf("JWT_SECRET wajib diisi saat APP_ENV=production")
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			env("DB_USER", "postgres"),
			env("DB_PASSWORD", "postgres"),
			env("DB_HOST", "localhost"),
			env("DB_PORT", "5432"),
			env("DB_NAME", "pos_food_db"),
		)
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	// Terima format Go ("15m", "168h") atau jumlah detik ("900").
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: format durasi tidak valid: %w", key, err)
	}
	return d, nil
}
