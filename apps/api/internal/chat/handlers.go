package chat

import (
	"log/slog"
	"net/http"
	"time"

	"compytition/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Team not found.")
		return
	}
	var before *time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "Invalid before timestamp.")
			return
		}
		before = &t
	}
	limit := httpx.QueryInt32(r, "limit", 50, 1, 100)
	messages, err := h.svc.History(r.Context(), u.ID, teamID, before, limit)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Team not found.")
		return
	}
	h.svc.ServeWS(w, r, u.ID, teamID)
}
