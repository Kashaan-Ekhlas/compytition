package admin

import (
	"log/slog"
	"net/http"

	"compytition/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	overview, err := h.svc.Overview(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, overview)
}

func (h *Handler) Seed(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		Events int   `json:"events"`
		Reset  *bool `json:"reset"`
	}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	// Deleting seed data must be explicit. A bare POST therefore adds to the
	// existing seed set instead of resetting it.
	reset := false
	if req.Reset != nil {
		reset = *req.Reset
	}
	result, err := h.svc.Seed(r.Context(), u.ID, SeedOptions{Events: req.Events, Reset: reset})
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	result, err := h.svc.Reset(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
