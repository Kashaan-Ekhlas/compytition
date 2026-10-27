package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"compytition/internal/apperr"
)

// WriteServiceError maps domain errors onto the consistent JSON error envelope.
func WriteServiceError(w http.ResponseWriter, log *slog.Logger, err error) {
	var ve *apperr.ValidationError
	switch {
	case errors.As(err, &ve):
		FieldError(w, ve.Fields)
	case errors.Is(err, apperr.ErrNotFound):
		Error(w, http.StatusNotFound, "Not found.")
	case errors.Is(err, apperr.ErrForbidden):
		Error(w, http.StatusForbidden, "You do not have permission to do that.")
	case errors.Is(err, apperr.ErrConflict):
		Error(w, http.StatusConflict, "That action conflicts with the current state.")
	case errors.Is(err, apperr.ErrUnauthorized):
		Error(w, http.StatusUnauthorized, "Sign in to continue.")
	case errors.Is(err, apperr.ErrRateLimited):
		Error(w, http.StatusTooManyRequests, "Too many attempts. Try again shortly.")
	case errors.Is(err, apperr.ErrClosed):
		Error(w, http.StatusConflict, "This is closed.")
	default:
		log.Error("request failed", "error", err)
		Error(w, http.StatusInternalServerError, "Something went wrong.")
	}
}
