package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"compytition/internal/auth"
	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/httpx"
)

type API struct {
	Store *db.Store
	Cfg   *config.Config
	Log   *slog.Logger
	Auth  *auth.Service
}

func NewRouter(api *API) http.Handler {
	mux := http.NewServeMux()
	authHandler := auth.NewHandler(api.Auth, api.Cfg, api.Log)
	authLimiter := NewRateLimiter(20, time.Minute)
	limitedAuth := func(h http.HandlerFunc) http.Handler {
		return RateLimit(authLimiter)(h)
	}

	// Health endpoints are unauthenticated and deliberately cheap.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := api.Store.Pool.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "Database is not ready.")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Auth
	mux.Handle("GET /api/v1/auth/config", limitedAuth(authHandler.Config))
	mux.Handle("GET /api/v1/auth/me", http.HandlerFunc(authHandler.Me))
	mux.Handle("POST /api/v1/auth/register", limitedAuth(authHandler.Register))
	mux.Handle("POST /api/v1/auth/login", limitedAuth(authHandler.Login))
	mux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(authHandler.Logout))
	mux.Handle("GET /api/v1/auth/username-available", limitedAuth(authHandler.UsernameAvailable))
	mux.Handle("PATCH /api/v1/auth/username", RequireAuth(http.HandlerFunc(authHandler.UpdateUsername)))
	mux.Handle("PATCH /api/v1/auth/onboarding", RequireAuth(http.HandlerFunc(authHandler.UpdateOnboarding)))
	mux.Handle("POST /api/v1/auth/password", RequireAuth(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("GET /api/v1/auth/google/start", limitedAuth(authHandler.GoogleStart))
	mux.Handle("GET /api/v1/auth/google/callback", limitedAuth(authHandler.GoogleCallback))

	// Profile
	mux.Handle("GET /api/v1/profile", RequireAuth(http.HandlerFunc(authHandler.MeProfile)))
	mux.Handle("PUT /api/v1/profile", RequireAuth(http.HandlerFunc(authHandler.UpdateProfile)))

	// Feature routes are registered by the domain modules below.
	registerEventRoutes(mux, api)
	registerTeamRoutes(mux, api)
	registerRegistrationRoutes(mux, api)
	registerSubmissionRoutes(mux, api)
	registerJudgingRoutes(mux, api)
	registerDashboardRoutes(mux, api)
	registerChatRoutes(mux, api)
	registerSupportRoutes(mux, api)

	return Chain(mux,
		RequestID,
		Logging(api.Log),
		Recover(api.Log),
		SecurityHeaders,
		CORS(api.Cfg),
		CSRF(api.Cfg),
		RateLimit(NewRateLimiter(api.Cfg.RateLimitPerMin, time.Minute)),
		Auth(api.Auth, api.Log),
	)
}

func (a *API) logger() *slog.Logger { return a.Log }
