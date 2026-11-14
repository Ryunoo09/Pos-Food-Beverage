package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv mengisi variabel dari .env terdekat (current dir lalu parent).
// Variabel yang sudah ada di environment OS tidak ditimpa, jadi produksi tetap
// bisa override lewat env. File .env tidak wajib ada.
func loadDotEnv() {
	for _, path := range []string{".env", filepath.Join("..", ".env")} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		readDotEnv(f)
		return
	}
}

// readDotEnv memformat sederhana: KEY=VALUE per baris, # untuk komentar,
// tanda kutip di awal/akhir nilai dibuang. Tidak mendukung interpolasi.
func readDotEnv(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}

		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"'`))
	}
}
