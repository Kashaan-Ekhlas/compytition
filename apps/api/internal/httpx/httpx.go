package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is the authenticated principal attached to a request by the session
// middleware. It is intentionally small; full profile data is fetched only
// where it is needed.
type User struct {
	ID                  uuid.UUID
	Email               string
	Username            string
	OnboardingCompleted bool
	OnboardingStep      int16
	Intents             []string
	IsAdmin             bool
	SessionID           uuid.UUID
}

type contextKey string

const userContextKey contextKey = "compytition.user"

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func UserFrom(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok
}

func MustUser(ctx context.Context) *User {
	u, ok := UserFrom(ctx)
	if !ok {
		panic("MustUser called without an authenticated user")
	}
	return u
}

// ── Error envelope ──────────────────────────────────────────────────────────

type ErrorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type errorEnvelope struct {
	Error ErrorPayload `json:"error"`
}

var statusCodes = map[int]string{
	http.StatusBadRequest:          "bad_request",
	http.StatusUnauthorized:        "unauthorized",
	http.StatusForbidden:           "forbidden",
	http.StatusNotFound:            "not_found",
	http.StatusConflict:            "conflict",
	http.StatusUnprocessableEntity: "validation_failed",
	http.StatusTooManyRequests:     "rate_limited",
	http.StatusInternalServerError: "internal_error",
	http.StatusServiceUnavailable:  "unavailable",
}

func Error(w http.ResponseWriter, status int, message string) {
	code, ok := statusCodes[status]
	if !ok {
		code = "error"
	}
	WriteJSON(w, status, errorEnvelope{Error: ErrorPayload{Code: code, Message: message}})
}

func ErrorCode(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorEnvelope{Error: ErrorPayload{Code: code, Message: message}})
}

func FieldError(w http.ResponseWriter, fields map[string]string) {
	WriteJSON(w, http.StatusUnprocessableEntity, errorEnvelope{Error: ErrorPayload{
		Code:    "validation_failed",
		Message: "Some fields need attention.",
		Fields:  fields,
	}})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// ── Request helpers ─────────────────────────────────────────────────────────

// DecodeJSON strictly decodes a single JSON object, rejecting unknown fields
// and silently allowing an empty body only when dst is nil.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return fmt.Errorf("request body is too large")
		case errors.Is(err, io.EOF):
			return fmt.Errorf("request body is required")
		default:
			return fmt.Errorf("invalid JSON: %v", err)
		}
	}
	if dec.More() {
		return fmt.Errorf("only one JSON object is allowed")
	}
	return nil
}

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := r.PathValue(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s", name)
	}
	return id, nil
}

func QueryInt(r *http.Request, name string, fallback, min, max int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func QueryInt32(r *http.Request, name string, fallback int32, min, max int32) int32 {
	return int32(QueryInt(r, name, int(fallback), int(min), int(max)))
}

func Pagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int32) {
	limit = int32(QueryInt(r, "limit", defaultLimit, 1, maxLimit))
	offset = int32(QueryInt(r, "offset", 0, 0, 1_000_000))
	return
}

// ParseTime accepts RFC3339 timestamps. Empty string returns nil.
func ParseTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("must be an RFC3339 timestamp")
	}
	utc := t.UTC()
	return &utc, nil
}

// IsValidHTTPURL accepts only absolute http(s) URLs without userinfo.
func IsValidHTTPURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if len(raw) > 2048 {
		return false
	}
	if strings.HasPrefix(raw, "javascript:") || strings.HasPrefix(raw, "data:") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
