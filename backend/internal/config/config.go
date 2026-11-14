package config

import (
	"fmt"
	"os"
	"time"
)

// Config hanya memuat nilai yang sudah dipakai. Nilai lain (JWT, backup)
// ditambahkan saat fiturnya masuk, supaya tidak ada config mati.
type Config struct {
	Env             string
	Port            string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Env:             env("APP_ENV", "development"),
		Port:            env("PORT", "8080"),
		DatabaseURL:     env("DATABASE_URL", ""),
		ShutdownTimeout: 10 * time.Second,
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			env("DB_USER", "pos_user"),
			env("DB_PASSWORD", ""),
			env("DB_HOST", "localhost"),
			env("DB_PORT", "5432"),
			env("DB_NAME", "pos_fb"),
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
