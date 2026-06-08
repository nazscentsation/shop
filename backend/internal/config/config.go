package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	Environment string
	AllowOrigin string
	ComingSoon  bool
	SiteURL     string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	AdminEmail  string
	TwilioSID   string
	TwilioToken string
	TwilioFrom  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		Environment: getEnv("ENVIRONMENT", "development"),
		AllowOrigin: getEnv("ALLOW_ORIGIN", "http://localhost:3000"),
		ComingSoon:  getEnv("COMING_SOON", "true") != "false",
		SiteURL:     getEnv("SITE_URL", "http://localhost:8080"),
		SMTPHost:    getEnv("SMTP_HOST", ""),
		SMTPPort:    getEnv("SMTP_PORT", "587"),
		SMTPUser:    getEnv("SMTP_USER", ""),
		SMTPPass:    getEnv("SMTP_PASS", ""),
		SMTPFrom:    getEnv("SMTP_FROM", ""),
		AdminEmail:  getEnv("ADMIN_EMAIL", ""),
		TwilioSID:   getEnv("TWILIO_SID", ""),
		TwilioToken: getEnv("TWILIO_TOKEN", ""),
		TwilioFrom:  getEnv("TWILIO_FROM", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
