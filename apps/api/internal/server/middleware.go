package server

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"compytition/internal/auth"
	"compytition/internal/config"
	"compytition/internal/httpx"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// redactPath removes bearer-like secrets that appear in URL paths, chiefly the
// staff invite token, before a path is written to the request log.
func redactPath(path string) string {
	const staffInvite = "/api/v1/invites/staff/"
	if !strings.HasPrefix(path, staffInvite) {
		return path
	}
	rest := strings.TrimPrefix(path, staffInvite)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return staffInvite + "***" + rest[i:]
	}
	return staffInvite + "***"
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Hijack keeps WebSocket upgrades working through the logging middleware.
// gorilla/websocket asserts http.Hijacker directly on the writer it receives.
func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	if w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return hj.Hijack()
}

func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			} else if rec.status >= 400 {
				level = slog.LevelWarn
			}
			log.Log(r.Context(), level, "request",
				"method", r.Method, "path", redactPath(r.URL.Path), "status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFrom(r.Context()), "ip", ClientIP(r),
			)
		})
	}
}

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic recovered", "panic", p, "request_id", RequestIDFrom(r.Context()),
						"stack", string(debug.Stack()))
					httpx.Error(w, http.StatusInternalServerError, "Something went wrong.")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// CORS allows exactly one browser origin (the Next.js app) with credentials.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && origin == cfg.WebOrigin {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Requested-With")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				if origin != "" && origin != cfg.WebOrigin {
					httpx.Error(w, http.StatusForbidden, "Origin not allowed.")
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF guards state-changing requests by validating the Origin/Referer against
// the configured web origin. SameSite=Lax cookies add a second layer.
func CSRF(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
				httpx.Error(w, http.StatusForbidden, "Cross-site request blocked.")
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
			}
			if origin != "" && !sameOrigin(origin, cfg.WebOrigin) {
				httpx.Error(w, http.StatusForbidden, "Origin not allowed.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameOrigin(candidate, allowed string) bool {
	candidate = strings.TrimRight(candidate, "/")
	if candidate == allowed {
		return true
	}
	return strings.HasPrefix(candidate, allowed+"/")
}

func ClientIP(r *http.Request) string {
	// Trust proxy headers only when the direct peer is a local/private address
	// (the Next.js rewrite proxy or a local load balancer).
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}
	peer := net.ParseIP(peerHost)
	if peer != nil && (peer.IsLoopback() || peer.IsPrivate()) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip.String()
			}
		}
	}
	return peerHost
}

// RateLimiter is a fixed-window limiter keyed by client IP. It is intentionally
// in-process and simple; for horizontal scaling this moves to a shared store.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateEntry
	lastGC  time.Time
}

type rateEntry struct {
	count int
	start time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, entries: map[string]*rateEntry{}, lastGC: time.Now()}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastGC) > 5*time.Minute {
		for k, e := range l.entries {
			if now.Sub(e.start) > 2*l.window {
				delete(l.entries, k)
			}
		}
		l.lastGC = now
	}
	e, ok := l.entries[key]
	if !ok || now.Sub(e.start) >= l.window {
		l.entries[key] = &rateEntry{count: 1, start: now}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}

func RateLimit(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(ClientIP(r)) {
				w.Header().Set("Retry-After", "60")
				httpx.Error(w, http.StatusTooManyRequests, "Too many requests. Slow down for a minute.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Auth loads the server-side session if a cookie is present. It never rejects;
// handlers that need a user call RequireAuth.
func Auth(svc *auth.Service, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.SessionCookie)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, err := svc.SessionUser(r.Context(), cookie.Value)
			if err != nil {
				if err != auth.ErrUnauthenticated {
					log.Warn("session lookup failed", "error", err)
				}
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(httpx.WithUser(r.Context(), user)))
		})
	}
}

// RequireAuth rejects anonymous requests with 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := httpx.UserFrom(r.Context()); !ok {
			httpx.Error(w, http.StatusUnauthorized, "Sign in to continue.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOnboarded is for actions that should wait until onboarding completes.
func RequireOnboarded(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := httpx.UserFrom(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "Sign in to continue.")
			return
		}
		if !u.OnboardingCompleted {
			httpx.ErrorCode(w, http.StatusForbidden, "onboarding_incomplete", "Finish onboarding first.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
