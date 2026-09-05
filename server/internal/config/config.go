package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	// AllowedOrigins berisi lebih dari satu entri karena "localhost" dan
	// "127.0.0.1" adalah origin yang BERBEDA di mata browser, walau menunjuk
	// mesin yang sama. Menyebut keduanya membuat aplikasi tetap jalan apa pun
	// yang diketik di address bar.
	AllowedOrigins []string
	SecureCookie   bool
}

const defaultOrigins = "http://localhost:5174,http://127.0.0.1:5174,http://[::1]:5174"

func Load() (Config, error) {
	c := Config{
		DatabaseURL:    env("DATABASE_URL", "postgres://chat:chat@localhost:5433/chatapp?sslmode=disable"),
		HTTPAddr:       env("HTTP_ADDR", ":8090"),
		AllowedOrigins: splitOrigins(env("ALLOWED_ORIGIN", defaultOrigins)),
	}

	if len(c.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_ORIGIN kosong")
	}

	secure, err := strconv.ParseBool(env("SECURE_COOKIE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("SECURE_COOKIE: %w", err)
	}
	c.SecureCookie = secure

	return c, nil
}

// AllowsOrigin melaporkan apakah origin permintaan termasuk yang diizinkan.
func (c Config) AllowsOrigin(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func splitOrigins(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
