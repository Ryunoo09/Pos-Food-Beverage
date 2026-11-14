package config

import "testing"

func TestLoad_BuildsDSNFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_USER", "pos_user")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "pos_fb")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := "postgres://pos_user:secret@db:5432/pos_fb?sslmode=disable"
	if cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

func TestLoad_DatabaseURLWins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom@localhost:5432/custom")
	t.Setenv("DB_USER", "ignored")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://custom@localhost:5432/custom" {
		t.Errorf("DatabaseURL = %q, want DATABASE_URL verbatim", cfg.DatabaseURL)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
}
