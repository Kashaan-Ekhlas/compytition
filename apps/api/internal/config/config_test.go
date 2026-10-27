package config

import "testing"

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("PORT", "8080")
	t.Setenv("WEB_ORIGIN", "http://localhost:3000")
	t.Setenv("SESSION_TTL_HOURS", "720")
	t.Setenv("RATE_LIMIT_PER_MIN", "240")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("AUTO_MIGRATE", "true")
}

func TestLoadAcceptsValidConfiguration(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" || cfg.RateLimitPerMin != 240 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.AutoMigrate || cfg.CookieSecure {
		t.Fatalf("unexpected booleans: auto=%v secure=%v", cfg.AutoMigrate, cfg.CookieSecure)
	}
}

func TestLoadRejectsMalformedNumbers(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SESSION_TTL_HOURS", "twenty-four")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed SESSION_TTL_HOURS to fail")
	}

	setValidEnv(t)
	t.Setenv("RATE_LIMIT_PER_MIN", "many")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed RATE_LIMIT_PER_MIN to fail")
	}
}

func TestLoadRejectsMalformedBoolean(t *testing.T) {
	setValidEnv(t)
	t.Setenv("COOKIE_SECURE", "yes-please")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed COOKIE_SECURE to fail")
	}
}

func TestLoadRejectsBadOriginAndPort(t *testing.T) {
	setValidEnv(t)
	t.Setenv("WEB_ORIGIN", "httpfoo")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed WEB_ORIGIN to fail")
	}

	setValidEnv(t)
	t.Setenv("WEB_ORIGIN", "http://localhost:3000/app")
	if _, err := Load(); err == nil {
		t.Fatal("expected WEB_ORIGIN with a path to fail")
	}

	setValidEnv(t)
	t.Setenv("PORT", "http")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed PORT to fail")
	}
}
