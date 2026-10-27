package registrations

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"compytition/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r, "event")
	if !ok {
		return
	}
	var req struct {
		TeamID  *uuid.UUID     `json:"team_id"`
		Answers map[string]any `json:"answers"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 128<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	reg, err := h.svc.Create(r.Context(), u.ID, eventID, req.TeamID, req.Answers)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"registration": reg})
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r, "event")
	if !ok {
		return
	}
	reg, err := h.svc.GetMine(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"registration": reg})
}

func (h *Handler) UpdateAnswers(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r, "event")
	if !ok {
		return
	}
	var req struct {
		Answers map[string]any `json:"answers"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 128<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	reg, err := h.svc.UpdateAnswers(r.Context(), u.ID, eventID, req.Answers)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"registration": reg})
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r, "event")
	if !ok {
		return
	}
	if err := h.svc.Cancel(r.Context(), u.ID, eventID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) ListForEvent(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r, "event")
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r, 50, 200)
	list, total, err := h.svc.ListForEvent(r.Context(), u.ID, eventID, r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"registrations": list, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) SetStatus(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	regID, ok := pathID(w, r, "registration")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 4<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.SetStatus(r.Context(), u.ID, regID, req.Status); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": req.Status})
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListMine(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"registrations": list})
}

func pathID(w http.ResponseWriter, r *http.Request, kind string) (uuid.UUID, bool) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Not found.")
		return uuid.Nil, false
	}
	return id, true
}
