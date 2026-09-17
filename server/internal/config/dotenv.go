package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// dotenvSearchDepth adalah berapa tingkat direktori ke atas yang diperiksa.
// Server biasa dijalankan dari `server/`, sementara `.env` tinggal di akar
// repo — satu tingkat di atasnya. Dua tingkat masih menjangkau `server/cmd`.
const dotenvSearchDepth = 3

// loadDotEnv membaca berkas `.env` terdekat, dari direktori kerja ke atas, dan
// memasang isinya ke lingkungan proses.
//
// Variabel yang SUDAH ada di lingkungan tidak pernah ditimpa. Urutannya
// sengaja begitu: di produksi, nilai datang dari orkestrator, dan berkas yang
// kebetulan tertinggal di disk tidak boleh diam-diam mengalahkannya. Di mesin
// pengembang, `.env` cukup untuk menyalakan lampiran, email, dan yang lain
// tanpa mengetik satu variabel pun di depan `go run`.
//
// Mengembalikan jalur berkas yang dibaca, atau "" bila tidak ada.
func loadDotEnv() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < dotenvSearchDepth; i++ {
		path := filepath.Join(dir, ".env")
		if f, err := os.Open(path); err == nil {
			applyDotEnv(f)
			f.Close()
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// applyDotEnv mengurai baris `KUNCI=nilai`. Yang didukung hanya bentuk yang
// memang dipakai `.env.example`: komentar `#`, baris kosong, awalan `export`,
// dan nilai yang boleh dibungkus tanda kutip. Tidak ada substitusi `${...}` —
// berkas ini tempat menyimpan nilai, bukan bahasa skrip.
func applyDotEnv(f *os.File) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, value)
	}
}

func parseDotEnvLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", "", false
	}

	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return key, value[1 : len(value)-1], true
	}
	// Komentar di ujung baris hanya berlaku bila didahului spasi, supaya
	// nilai seperti "http://host/#jangkar" tetap utuh.
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return key, value, true
}
