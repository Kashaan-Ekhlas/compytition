package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/httpx"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("too many failed attempts; try again later")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrGoogleDisabled     = errors.New("google sign-in is not configured")
	ErrUnauthenticated    = errors.New("not authenticated")
)

var (
	emailRE    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)
)

const (
	loginWindow      = 15 * time.Minute
	maxLoginFailures = 8
	loginLock        = 15 * time.Minute
)

var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "api": true, "root": true,
	"support": true, "compytition": true, "system": true, "judge": true,
	"host": true, "events": true, "dashboard": true, "profile": true,
	"settings": true, "login": true, "register": true, "onboarding": true,
}

var allowedIntents = map[string]bool{"find": true, "host": true, "judge": true}

// FieldErrors is surfaced to clients as the `fields` object in the error
// envelope. Keys are request field names.
type FieldErrors map[string]string

type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string { return "validation failed" }

type SessionResult struct {
	Token     string
	ExpiresAt time.Time
	User      *httpx.User
}

type Service struct {
	store     *db.Store
	cfg       *config.Config
	log       *slog.Logger
	dummyHash string

	googleOnce         sync.Once
	googleProviderInst *oidc.Provider
	googleErr          error
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	s := &Service{store: store, cfg: cfg, log: log}
	// A valid hash used to equalise timing when the email does not exist.
	if h, err := HashPassword("this-password-is-never-valid-000"); err == nil {
		s.dummyHash = h
	}
	return s
}

// ── Registration & login ────────────────────────────────────────────────────

func (s *Service) Register(ctx context.Context, email, password, username string) (*SessionResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	fields := FieldErrors{}
	if !validEmail(email) {
		fields["email"] = "Enter a valid email address."
	}
	if msg := validatePassword(password); msg != "" {
		fields["password"] = msg
	}
	if username == "" {
		var err error
		username, err = s.availableUsername(ctx, usernameSeed(email))
		if err != nil {
			return nil, err
		}
	} else if msg := validateChosenUsername(username); msg != "" {
		fields["username"] = msg
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	var created db.User
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		u, err := q.CreateUser(ctx, db.CreateUserParams{Email: email, Username: username})
		if err != nil {
			return mapPGError(err)
		}
		created = u
		if err := q.CreateIdentity(ctx, db.CreateIdentityParams{
			UserID: u.ID, Provider: "password", ProviderUserID: email, SecretHash: &hash,
		}); err != nil {
			return err
		}
		if _, err := q.UpsertProfile(ctx, db.UpsertProfileParams{UserID: u.ID}); err != nil {
			return err
		}
		if _, err := q.CreateSession(ctx, db.CreateSessionParams{
			UserID: u.ID, TokenHash: tokenHash, ExpiresAt: expires,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	user := &httpx.User{
		ID: created.ID, Email: created.Email, Username: created.Username,
		OnboardingStep: created.OnboardingStep, Intents: []string{},
	}
	return &SessionResult{Token: token, ExpiresAt: expires, User: user}, nil
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (*SessionResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	attemptKey := loginKey(ip, email)
	if rec, err := s.store.GetLoginAttempt(ctx, attemptKey); err == nil {
		if rec.LockedUntil != nil && rec.LockedUntil.After(time.Now()) {
			return nil, ErrAccountLocked
		}
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		// Equalise timing and record the failure.
		if s.dummyHash != "" {
			_, _ = VerifyPassword(password, s.dummyHash)
		}
		s.recordFailure(ctx, attemptKey)
		return nil, ErrInvalidCredentials
	}
	identity, err := s.store.GetIdentityForUser(ctx, db.GetIdentityForUserParams{UserID: user.ID, Provider: "password"})
	if err != nil || identity.SecretHash == nil {
		s.recordFailure(ctx, attemptKey)
		return nil, ErrInvalidCredentials
	}
	ok, err := VerifyPassword(password, *identity.SecretHash)
	if err != nil || !ok {
		s.recordFailure(ctx, attemptKey)
		return nil, ErrInvalidCredentials
	}
	_ = s.store.ClearLoginAttempts(ctx, attemptKey)

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	if _, err := s.store.CreateSession(ctx, db.CreateSessionParams{
		UserID: user.ID, TokenHash: tokenHash, ExpiresAt: expires,
	}); err != nil {
		return nil, err
	}
	intents := parseIntents(user.Intents)
	return &SessionResult{Token: token, ExpiresAt: expires, User: &httpx.User{
		ID: user.ID, Email: user.Email, Username: user.Username,
		OnboardingCompleted: user.OnboardingCompleted, OnboardingStep: user.OnboardingStep,
		Intents: intents, IsAdmin: user.IsAdmin || s.cfg.IsAdminEmail(user.Email),
	}}, nil
}

func (s *Service) recordFailure(ctx context.Context, key string) {
	attempts := 1
	rec, err := s.store.GetLoginAttempt(ctx, key)
	if err == nil && time.Since(rec.FirstAttemptAt) < loginWindow {
		attempts = int(rec.Attempts) + 1
	}
	var lock *time.Time
	if attempts >= maxLoginFailures {
		t := time.Now().Add(loginLock)
		lock = &t
	}
	if _, err := s.store.RecordLoginFailure(ctx, db.RecordLoginFailureParams{Key: key, LockedUntil: lock}); err != nil {
		s.log.Warn("could not record login failure", "error", err)
	}
}

// ── Sessions ────────────────────────────────────────────────────────────────

func (s *Service) SessionUser(ctx context.Context, rawToken string) (*httpx.User, error) {
	if rawToken == "" {
		return nil, ErrUnauthenticated
	}
	row, err := s.store.GetSessionByHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthenticated
		}
		return nil, err
	}
	_ = s.store.TouchSession(ctx, row.ID)
	return &httpx.User{
		ID: row.UserID, Email: row.Email, Username: row.Username,
		OnboardingCompleted: row.OnboardingCompleted, OnboardingStep: row.OnboardingStep,
		Intents: parseIntents(row.Intents), IsAdmin: row.IsAdmin || s.cfg.IsAdminEmail(row.Email),
		SessionID: row.ID,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(rawToken))
}

// ── Account settings ────────────────────────────────────────────────────────

func (s *Service) UsernameAvailable(ctx context.Context, username string, viewerID *uuid.UUID) (bool, string) {
	username = strings.TrimSpace(username)
	if msg := validateChosenUsername(username); msg != "" {
		return false, msg
	}
	existing, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, ""
		}
		return false, "Could not check that username right now."
	}
	if viewerID != nil && existing.ID == *viewerID {
		return true, ""
	}
	return false, "That username is taken."
}

func (s *Service) UpdateUsername(ctx context.Context, userID uuid.UUID, username string) error {
	username = strings.TrimSpace(username)
	if msg := validateChosenUsername(username); msg != "" {
		return &ValidationError{Fields: FieldErrors{"username": msg}}
	}
	current, err := s.store.GetUserByID(ctx, userID)
	if err == nil && current.Username == username {
		return nil
	}
	if err := s.store.UpdateUsername(ctx, db.UpdateUsernameParams{ID: userID, Username: username}); err != nil {
		if isUniqueViolation(err) {
			return &ValidationError{Fields: FieldErrors{"username": "That username is taken."}}
		}
		return err
	}
	return nil
}

func (s *Service) UpdateOnboarding(ctx context.Context, userID uuid.UUID, step int16, completed bool, intents []string) error {
	fields := FieldErrors{}
	if step < 1 || step > 7 {
		fields["step"] = "Step must be between 1 and 7."
	}
	clean := make([]string, 0, len(intents))
	for _, in := range intents {
		in = strings.ToLower(strings.TrimSpace(in))
		if !allowedIntents[in] {
			fields["intents"] = "Unknown intent."
			break
		}
		clean = append(clean, in)
	}
	if completed {
		user, err := s.store.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		if isPlaceholderUsername(user.Username) {
			fields["username"] = "Choose a username before finishing onboarding."
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	return s.store.UpdateOnboarding(ctx, db.UpdateOnboardingParams{
		ID: userID, OnboardingStep: step, OnboardingCompleted: completed, Intents: raw,
	})
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentSessionID uuid.UUID, current, next string) error {
	fields := FieldErrors{}
	if msg := validatePassword(next); msg != "" {
		fields["new_password"] = msg
	} else if current == next {
		fields["new_password"] = "The new password must be different."
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	identity, err := s.store.GetIdentityForUser(ctx, db.GetIdentityForUserParams{UserID: userID, Provider: "password"})
	if err != nil || identity.SecretHash == nil {
		return ErrInvalidCredentials
	}
	ok, err := VerifyPassword(current, *identity.SecretHash)
	if err != nil || !ok {
		return &ValidationError{Fields: FieldErrors{"current_password": "Current password is incorrect."}}
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.store.UpdateIdentitySecret(ctx, db.UpdateIdentitySecretParams{ID: identity.ID, SecretHash: &hash}); err != nil {
		return err
	}
	return s.store.DeleteOtherSessions(ctx, db.DeleteOtherSessionsParams{UserID: userID, ID: currentSessionID})
}

func (s *Service) Profile(ctx context.Context, userID uuid.UUID) (*db.Profile, *db.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &db.Profile{UserID: userID}, &user, nil
		}
		return nil, nil, err
	}
	return &profile, &user, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, fullName, college, course, country, state, city string, year *int16) (*db.Profile, error) {
	fields := FieldErrors{}
	fullName = strings.TrimSpace(fullName)
	college = strings.TrimSpace(college)
	course = strings.TrimSpace(course)
	country = strings.TrimSpace(country)
	state = strings.TrimSpace(state)
	city = strings.TrimSpace(city)
	if len(fullName) > 120 {
		fields["full_name"] = "Keep this under 120 characters."
	}
	if len(college) > 160 {
		fields["college"] = "Keep this under 160 characters."
	}
	if len(course) > 120 {
		fields["course"] = "Keep this under 120 characters."
	}
	if len(country) > 80 || len(state) > 80 || len(city) > 80 {
		fields["location"] = "Keep each location field under 80 characters."
	}
	if year != nil && (*year < 1 || *year > 8) {
		fields["year_of_study"] = "Year must be between 1 and 8."
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	p, err := s.store.UpsertProfile(ctx, db.UpsertProfileParams{
		UserID: userID, FullName: fullName, College: college, Course: course, YearOfStudy: year,
		Country: country, State: state, City: city,
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func validatePassword(pw string) string {
	if len(pw) < 8 {
		return "Use at least 8 characters."
	}
	if len(pw) > 128 {
		return "Use at most 128 characters."
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "Include at least one letter and one number."
	}
	return ""
}

func validateUsername(username string) string {
	if !usernameRE.MatchString(username) {
		return "Use 3–30 letters, numbers or underscores."
	}
	if reservedUsernames[strings.ToLower(username)] {
		return "That username is reserved."
	}
	return ""
}

// validateChosenUsername additionally blocks the auto-generated placeholder
// pattern, so users cannot select a name that onboarding would later reject.
func validateChosenUsername(username string) string {
	if msg := validateUsername(username); msg != "" {
		return msg
	}
	if isPlaceholderUsername(username) {
		return "Choose a username that is not auto-generated."
	}
	return ""
}

func validEmail(email string) bool {
	return len(email) <= 254 && emailRE.MatchString(email)
}

func loginKey(ip, email string) string {
	sum := sha256.Sum256([]byte(ip + "|" + strings.ToLower(email)))
	return hex.EncodeToString(sum[:])
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func newSessionToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func parseIntents(raw []byte) []string {
	var intents []string
	if len(raw) == 0 {
		return []string{}
	}
	if err := json.Unmarshal(raw, &intents); err != nil || intents == nil {
		return []string{}
	}
	return intents
}

func usernameSeed(email string) string {
	local := email
	if i := strings.IndexByte(email, '@'); i > 0 {
		local = email[:i]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(local) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	seed := strings.Trim(b.String(), "_")
	if len(seed) < 3 {
		seed = "user"
	}
	if len(seed) > 20 {
		seed = seed[:20]
	}
	return seed
}

func (s *Service) availableUsername(ctx context.Context, seed string) (string, error) {
	for i := 0; i < 20; i++ {
		candidate := seed
		if i > 0 {
			candidate = fmt.Sprintf("%s_%s", seed, randomSuffix(4))
		}
		if msg := validateUsername(candidate); msg != "" {
			candidate = "user_" + randomSuffix(6)
		}
		taken, err := s.store.UsernameTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", errors.New("could not allocate a username")
}

func isPlaceholderUsername(username string) bool {
	if !strings.HasPrefix(username, "user_") || len(username) != 11 {
		return false
	}
	for _, r := range username[5:] {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func randomSuffix(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:n]
	}
	return hex.EncodeToString(b)[:n]
}

func mapPGError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return &ValidationError{Fields: FieldErrors{"email": "An account with that email already exists."}}
		case "users_username_key":
			return &ValidationError{Fields: FieldErrors{"username": "That username is taken."}}
		default:
			return &ValidationError{Fields: FieldErrors{"email": "An account with those details already exists."}}
		}
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
