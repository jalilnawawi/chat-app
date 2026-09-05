package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	AllowedOrigin string
	SecureCookie  bool
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:   env("DATABASE_URL", "postgres://chat:chat@localhost:5433/chatapp?sslmode=disable"),
		HTTPAddr:      env("HTTP_ADDR", ":8090"),
		AllowedOrigin: env("ALLOWED_ORIGIN", "http://localhost:5174"),
	}

	secure, err := strconv.ParseBool(env("SECURE_COOKIE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("SECURE_COOKIE: %w", err)
	}
	c.SecureCookie = secure

	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
