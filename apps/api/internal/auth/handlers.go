package auth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/httpx"
)

const SessionCookie = "compytition_session"

type Handler struct {
	svc *Service
	cfg *config.Config
	log *slog.Logger
}

func NewHandler(svc *Service, cfg *config.Config, log *slog.Logger) *Handler {
	return &Handler{svc: svc, cfg: cfg, log: log}
}

type userDTO struct {
	ID                  uuid.UUID `json:"id"`
	Email               string    `json:"email"`
	Username            string    `json:"username"`
	OnboardingStep      int16     `json:"onboarding_step"`
	OnboardingCompleted bool      `json:"onboarding_completed"`
	Intents             []string  `json:"intents"`
	IsAdmin             bool      `json:"is_admin"`
}

func toUserDTO(u *httpx.User) userDTO {
	intents := u.Intents
	if intents == nil {
		intents = []string{}
	}
	return userDTO{
		ID: u.ID, Email: u.Email, Username: u.Username,
		OnboardingStep: u.OnboardingStep, OnboardingCompleted: u.OnboardingCompleted,
		Intents: intents, IsAdmin: u.IsAdmin,
	}
}

func profileDTO(p *db.Profile) map[string]any {
	if p == nil {
		return nil
	}
	var year *int16
	if p.YearOfStudy != nil {
		year = p.YearOfStudy
	}
	return map[string]any{
		"full_name": p.FullName, "college": p.College, "course": p.Course,
		"country": p.Country, "state": p.State, "city": p.City,
		"year_of_study": year, "updated_at": p.UpdatedAt,
	}
}

// ── Auth flow ───────────────────────────────────────────────────────────────

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 16<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.svc.Register(r.Context(), req.Email, req.Password, req.Username)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.setSessionCookie(w, res.Token, res.ExpiresAt)
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"user": toUserDTO(res.User)})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 16<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password, clientIP(r))
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.setSessionCookie(w, res.Token, res.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(res.User)})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if err := h.svc.Logout(r.Context(), token); err != nil {
		h.log.Error("logout failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "Could not log out.")
		return
	}
	h.clearSessionCookie(w)
	httpx.NoContent(w)
}

// Me returns the current user, or null when logged out. The onboarding screen
// depends on this endpoint returning 200 while unauthenticated.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := httpx.UserFrom(r.Context())
	if !ok {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"user": nil, "profile": nil, "google_enabled": h.svc.GoogleEnabled(),
		})
		return
	}
	profile, _, err := h.svc.Profile(r.Context(), u.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		h.log.Error("load profile", "error", err)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user": toUserDTO(u), "profile": profileDTO(profile),
		"google_enabled": h.svc.GoogleEnabled(),
	})
}

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"google_enabled": h.svc.GoogleEnabled(),
	})
}

func (h *Handler) MeProfile(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	profile, user, err := h.svc.Profile(r.Context(), u.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user": toUserDTO(&httpx.User{
			ID: user.ID, Email: user.Email, Username: user.Username,
			OnboardingStep: user.OnboardingStep, OnboardingCompleted: user.OnboardingCompleted,
			Intents: parseIntents(user.Intents),
			IsAdmin: user.IsAdmin || h.cfg.IsAdminEmail(user.Email),
		}),
		"profile": profileDTO(profile),
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		FullName    string `json:"full_name"`
		College     string `json:"college"`
		Course      string `json:"course"`
		Country     string `json:"country"`
		State       string `json:"state"`
		City        string `json:"city"`
		YearOfStudy *int16 `json:"year_of_study"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 16<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := h.svc.UpdateProfile(r.Context(), u.ID, req.FullName, req.College, req.Course, req.Country, req.State, req.City, req.YearOfStudy)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"profile": profileDTO(profile)})
}

func (h *Handler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		Username string `json:"username"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateUsername(r.Context(), u.ID, req.Username); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"username": strings.TrimSpace(req.Username)})
}

func (h *Handler) UsernameAvailable(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	var viewerID *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		viewerID = &u.ID
	}
	available, reason := h.svc.UsernameAvailable(r.Context(), username, viewerID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"available": available, "reason": reason})
}

func (h *Handler) UpdateOnboarding(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		Step      int16    `json:"step"`
		Completed bool     `json:"completed"`
		Intents   []string `json:"intents"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateOnboarding(r.Context(), u.ID, req.Step, req.Completed, req.Intents); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 16<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.ChangePassword(r.Context(), u.ID, u.SessionID, req.CurrentPassword, req.NewPassword); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.NoContent(w)
}

// ── Google OAuth ────────────────────────────────────────────────────────────

func (h *Handler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	url, err := h.svc.BeginGoogle(r.Context(), r.URL.Query().Get("redirect"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	res, redirect, err := h.svc.FinishGoogle(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"), clientIP(r))
	if err != nil {
		h.log.Warn("google sign-in failed", "error", err)
		http.Redirect(w, r, h.cfg.WebOrigin+"/login?error=google_failed", http.StatusSeeOther)
		return
	}
	h.setSessionCookie(w, res.Token, res.ExpiresAt)
	http.Redirect(w, r, h.cfg.WebOrigin+redirect, http.StatusSeeOther)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true,
		Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func sessionToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.FieldError(w, ve.Fields)
	case errors.Is(err, ErrInvalidCredentials):
		httpx.ErrorCode(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
	case errors.Is(err, ErrUnauthenticated):
		httpx.Error(w, http.StatusUnauthorized, "Sign in to continue.")
	case errors.Is(err, ErrAccountLocked):
		httpx.Error(w, http.StatusTooManyRequests, "Too many attempts. Try again in 15 minutes.")
	case errors.Is(err, ErrGoogleDisabled):
		httpx.Error(w, http.StatusServiceUnavailable, "Google sign-in is not configured.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "Not found.")
	default:
		h.log.Error("auth request failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "Something went wrong.")
	}
}
