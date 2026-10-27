package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"compytition/internal/db"
	"compytition/internal/httpx"
)

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
}

func (s *Service) GoogleEnabled() bool { return s.cfg.GoogleEnabled() }

func (s *Service) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.GoogleClientID,
		ClientSecret: s.cfg.GoogleSecret,
		RedirectURL:  s.cfg.GoogleRedirect,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (s *Service) googleProvider(ctx context.Context) (*oidc.Provider, error) {
	s.googleOnce.Do(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		s.googleProviderInst, s.googleErr = oidc.NewProvider(ctx, "https://accounts.google.com")
	})
	return s.googleProviderInst, s.googleErr
}

func (s *Service) BeginGoogle(ctx context.Context, redirectTo string) (string, error) {
	if !s.cfg.GoogleEnabled() {
		return "", ErrGoogleDisabled
	}
	if !safeRedirect(redirectTo) {
		redirectTo = "/dashboard"
	}
	state := randomURLToken(32)
	verifier := randomURLToken(48)
	if err := s.store.CreateOAuthState(ctx, db.CreateOAuthStateParams{
		State: state, CodeVerifier: verifier, RedirectTo: redirectTo,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}); err != nil {
		return "", err
	}
	return s.oauthConfig().AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	), nil
}

// FinishGoogle exchanges the authorization code, cryptographically verifies the
// Google ID token (signature, issuer, audience and expiry), then logs the user
// in. The returned path is the safe in-app redirect captured at start.
func (s *Service) FinishGoogle(ctx context.Context, code, state, ip string) (*SessionResult, string, error) {
	if !s.cfg.GoogleEnabled() {
		return nil, "", ErrGoogleDisabled
	}
	if code == "" || state == "" {
		return nil, "", errors.New("missing OAuth code or state")
	}
	saved, err := s.store.ConsumeOAuthState(ctx, state)
	if err != nil {
		return nil, "", errors.New("OAuth state is invalid or expired")
	}
	token, err := s.oauthConfig().Exchange(ctx, code, oauth2.VerifierOption(saved.CodeVerifier))
	if err != nil {
		return nil, "", fmt.Errorf("exchange Google code: %w", err)
	}
	rawID, _ := token.Extra("id_token").(string)
	if rawID == "" {
		return nil, "", errors.New("Google did not return an ID token")
	}
	provider, err := s.googleProvider(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("load Google OIDC provider: %w", err)
	}
	verified, err := provider.Verifier(&oidc.Config{ClientID: s.cfg.GoogleClientID}).Verify(ctx, rawID)
	if err != nil {
		return nil, "", fmt.Errorf("verify Google ID token: %w", err)
	}
	var claims googleClaims
	if err := verified.Claims(&claims); err != nil {
		return nil, "", fmt.Errorf("read Google claims: %w", err)
	}
	if claims.Email == "" || claims.Sub == "" {
		return nil, "", errors.New("Google account is missing an email")
	}
	if !claims.EmailVerified {
		return nil, "", errors.New("Google email is not verified")
	}

	userID, err := s.upsertGoogleUser(ctx, strings.ToLower(claims.Email), claims.Sub, claims.Name)
	if err != nil {
		return nil, "", err
	}
	tokenRaw, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, "", err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.store.CreateSession(ctx, db.CreateSessionParams{
		UserID: userID, TokenHash: tokenHash, ExpiresAt: expires,
	}); err != nil {
		return nil, "", err
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	_ = s.store.ClearLoginAttempts(ctx, loginKey(ip, claims.Email))
	redirect := saved.RedirectTo
	if !safeRedirect(redirect) {
		redirect = "/dashboard"
	}
	return &SessionResult{
		Token: tokenRaw, ExpiresAt: expires,
		User: &httpx.User{
			ID: user.ID, Email: user.Email, Username: user.Username,
			OnboardingCompleted: user.OnboardingCompleted, OnboardingStep: user.OnboardingStep,
			Intents: parseIntents(user.Intents),
			IsAdmin: user.IsAdmin || s.cfg.IsAdminEmail(user.Email),
		},
	}, redirect, nil
}

func (s *Service) upsertGoogleUser(ctx context.Context, email, sub, name string) (uuid.UUID, error) {
	identity, err := s.store.GetIdentity(ctx, db.GetIdentityParams{Provider: "google", ProviderUserID: sub})
	if err == nil {
		return identity.UserID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	user, err := s.store.GetUserByEmail(ctx, email)
	if err == nil {
		if err := s.store.CreateIdentity(ctx, db.CreateIdentityParams{
			UserID: user.ID, Provider: "google", ProviderUserID: sub,
		}); err != nil {
			if isUniqueViolation(err) {
				// A concurrent callback linked this identity first.
				linked, fetchErr := s.store.GetIdentity(ctx, db.GetIdentityParams{Provider: "google", ProviderUserID: sub})
				if fetchErr != nil {
					return uuid.Nil, fetchErr
				}
				return linked.UserID, nil
			}
			return uuid.Nil, err
		}
		return user.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	username, err := s.availableUsername(ctx, usernameSeed(email))
	if err != nil {
		return uuid.Nil, err
	}
	var created uuid.UUID
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		u, err := q.CreateUser(ctx, db.CreateUserParams{Email: email, Username: username})
		if err != nil {
			return mapPGError(err)
		}
		created = u.ID
		if err := q.CreateIdentity(ctx, db.CreateIdentityParams{
			UserID: u.ID, Provider: "google", ProviderUserID: sub,
		}); err != nil {
			return err
		}
		fullName := strings.TrimSpace(name)
		if len(fullName) > 120 {
			fullName = fullName[:120]
		}
		if _, err := q.UpsertProfile(ctx, db.UpsertProfileParams{UserID: u.ID, FullName: fullName}); err != nil {
			return err
		}
		return nil
	})
	return created, err
}

func safeRedirect(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "\r\n")
}

func randomURLToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
