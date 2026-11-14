package config

import (
	"os"
	"strings"
	"testing"
)

func TestReadDotEnv_SetsMissingAndKeepsOSEnv(t *testing.T) {
	const (
		newKey   = "POS_TEST_DOTENV_NEW"
		keepKey  = "POS_TEST_DOTENV_KEEP"
		quoteKey = "POS_TEST_DOTENV_QUOTED"
	)

	os.Unsetenv(newKey)
	t.Cleanup(func() { os.Unsetenv(newKey) })
	t.Setenv(keepKey, "dari-os")

	readDotEnv(strings.NewReader(`
# komentar diabaikan
` + newKey + `=dari-file
` + keepKey + `=dari-file
` + quoteKey + `="nilai ada spasi"
baris tanpa sama dengan
`))

	if got := os.Getenv(newKey); got != "dari-file" {
		t.Errorf("%s = %q, want dari-file", newKey, got)
	}
	if got := os.Getenv(keepKey); got != "dari-os" {
		t.Errorf("%s = %q, harus tetap nilai OS", keepKey, got)
	}
	if got := os.Getenv(quoteKey); got != "nilai ada spasi" {
		t.Errorf("%s = %q, tanda kutip harus dibuang", quoteKey, got)
	}
	os.Unsetenv(quoteKey)
}

func TestLoad_ReadsDotEnvFromCurrentDir(t *testing.T) {
	os.Unsetenv("PORT")
	t.Cleanup(func() { os.Unsetenv("PORT") })

	dir := t.TempDir()
	content := "PORT=9099\n"
	if err := os.WriteFile(dir+"/.env", []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "9099" {
		t.Errorf("Port = %q, want 9099 dari .env", cfg.Port)
	}
}
