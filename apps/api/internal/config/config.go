package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is loaded once at startup from environment variables. The process
// fails fast when a required value is missing or malformed.
type Config struct {
	Env             string
	Port            string
	DatabaseURL     string
	WebOrigin       string
	SessionTTL      time.Duration
	CookieSecure    bool
	AutoMigrate     bool
	GoogleClientID  string
	GoogleSecret    string
	GoogleRedirect  string
	AdminEmails     map[string]bool
	LogLevel        string
	RateLimitPerMin int
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:             env("APP_ENV", "development"),
		Port:            env("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		WebOrigin:       strings.TrimRight(env("WEB_ORIGIN", "http://localhost:3000"), "/"),
		SessionTTL:      time.Duration(envInt("SESSION_TTL_HOURS", 720)) * time.Hour,
		CookieSecure:    envBool("COOKIE_SECURE", false),
		AutoMigrate:     envBool("AUTO_MIGRATE", true),
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect:  env("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		AdminEmails:     parseEmails(os.Getenv("ADMIN_EMAILS")),
		LogLevel:        env("LOG_LEVEL", "info"),
		RateLimitPerMin: envInt("RATE_LIMIT_PER_MIN", 240),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Port == "" {
		return nil, fmt.Errorf("PORT must not be empty")
	}
	if cfg.WebOrigin == "" || !strings.HasPrefix(cfg.WebOrigin, "http") {
		return nil, fmt.Errorf("WEB_ORIGIN must be an absolute http(s) URL")
	}
	if cfg.SessionTTL < time.Hour {
		return nil, fmt.Errorf("SESSION_TTL_HOURS must be at least 1")
	}
	if cfg.RateLimitPerMin < 1 {
		return nil, fmt.Errorf("RATE_LIMIT_PER_MIN must be positive")
	}
	if cfg.Env == "production" && !cfg.CookieSecure {
		return nil, fmt.Errorf("COOKIE_SECURE must be true in production")
	}
	if (cfg.GoogleClientID == "") != (cfg.GoogleSecret == "") {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together")
	}
	return cfg, nil
}

func (c *Config) GoogleEnabled() bool { return c.GoogleClientID != "" && c.GoogleSecret != "" }

func (c *Config) IsAdminEmail(email string) bool {
	return c.AdminEmails[strings.ToLower(strings.TrimSpace(email))]
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func parseEmails(raw string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if e := strings.ToLower(strings.TrimSpace(part)); e != "" {
			out[e] = true
		}
	}
	return out
}
