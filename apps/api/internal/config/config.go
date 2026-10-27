package config

import (
	"fmt"
	"net/url"
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
	sessionTTLHours, err := envInt("SESSION_TTL_HOURS", 720)
	if err != nil {
		return nil, err
	}
	rateLimit, err := envInt("RATE_LIMIT_PER_MIN", 240)
	if err != nil {
		return nil, err
	}
	cookieSecure, err := envBool("COOKIE_SECURE", false)
	if err != nil {
		return nil, err
	}
	autoMigrate, err := envBool("AUTO_MIGRATE", true)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Env:             env("APP_ENV", "development"),
		Port:            env("PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		WebOrigin:       strings.TrimRight(env("WEB_ORIGIN", "http://localhost:3000"), "/"),
		SessionTTL:      time.Duration(sessionTTLHours) * time.Hour,
		CookieSecure:    cookieSecure,
		AutoMigrate:     autoMigrate,
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect:  env("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		AdminEmails:     parseEmails(os.Getenv("ADMIN_EMAILS")),
		LogLevel:        env("LOG_LEVEL", "info"),
		RateLimitPerMin: rateLimit,
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	if cfg.WebOrigin == "" || !validOrigin(cfg.WebOrigin) {
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

func envInt(key string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return n, nil
}

func envBool(key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}

func validOrigin(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" && (u.Path == "" || u.Path == "/") &&
		u.RawQuery == "" && u.Fragment == ""
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
