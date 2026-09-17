package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvLine(t *testing.T) {
	cases := []struct {
		line, key, value string
		ok               bool
	}{
		{"SEAWEED_FILER_URL=http://127.0.0.1:8890", "SEAWEED_FILER_URL", "http://127.0.0.1:8890", true},
		{"  export MAIL_FROM = chat@contoh.test  ", "MAIL_FROM", "chat@contoh.test", true},
		{`SMTP_URL="smtp://localhost:1025"`, "SMTP_URL", "smtp://localhost:1025", true},
		{"APP_URL=http://host/#jangkar", "APP_URL", "http://host/#jangkar", true},
		{"RATE_X=0.5 # setengah per menit", "RATE_X", "0.5", true},
		{"REDIS_URL=", "REDIS_URL", "", true},
		{"# REDIS_URL=redis://localhost", "", "", false},
		{"", "", "", false},
		{"tanpa-sama-dengan", "", "", false},
	}
	for _, c := range cases {
		key, value, ok := parseDotEnvLine(c.line)
		if key != c.key || value != c.value || ok != c.ok {
			t.Errorf("%q: dapat (%q, %q, %v), ingin (%q, %q, %v)", c.line, key, value, ok, c.key, c.value, c.ok)
		}
	}
}

// Berkas di akar repo ditemukan dari `server/`, dan lingkungan yang sudah ada
// tidak ditimpa.
func TestLoadDotEnvFromParentKeepsExisting(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "server")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "DOTENV_TEST_NEW=dari-berkas\nDOTENV_TEST_SET=dari-berkas\n"
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub)
	t.Setenv("DOTENV_TEST_SET", "dari-lingkungan")
	os.Unsetenv("DOTENV_TEST_NEW")
	t.Cleanup(func() { os.Unsetenv("DOTENV_TEST_NEW") })

	if got := loadDotEnv(); got != filepath.Join(root, ".env") {
		t.Fatalf("berkas yang dibaca %q", got)
	}
	if v := os.Getenv("DOTENV_TEST_NEW"); v != "dari-berkas" {
		t.Errorf("DOTENV_TEST_NEW = %q", v)
	}
	if v := os.Getenv("DOTENV_TEST_SET"); v != "dari-lingkungan" {
		t.Errorf("DOTENV_TEST_SET ditimpa jadi %q", v)
	}
}
