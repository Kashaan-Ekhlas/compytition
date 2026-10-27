package judging

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

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := eventID(w, r)
	if !ok {
		return
	}
	preview, err := h.svc.Preview(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, preview)
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	res, err := h.svc.Assign(r.Context(), u.ID, id)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Start(r.Context(), u.ID, id); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"judging_status": "live"})
}

func (h *Handler) Next(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	next, err := h.svc.Next(r.Context(), u.ID, id)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, next)
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	pairID, err := httpx.PathUUID(r, "pairId")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Comparison not found.")
		return
	}
	var req struct {
		Winner   string `json:"winner"`
		Skipped  bool   `json:"skipped"`
		Conflict bool   `json:"conflict"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Compare(r.Context(), u.ID, id, pairID, req.Winner, req.Skipped, req.Conflict); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	progress, _ := h.svc.Progress(r.Context(), u.ID, id)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"saved": true, "progress": progress})
}

func (h *Handler) Undo(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	pairID, err := httpx.PathUUID(r, "pairId")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Comparison not found.")
		return
	}
	if err := h.svc.Undo(r.Context(), u.ID, id, pairID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	progress, _ := h.svc.Progress(r.Context(), u.ID, id)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"undone": true, "progress": progress})
}

func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	progress, err := h.svc.Progress(r.Context(), u.ID, id)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, progress)
}

func (h *Handler) Review(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	review, err := h.svc.Review(r.Context(), u.ID, id)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, review)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	var req struct {
		Force bool `json:"force"`
	}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &req, 4<<10); err != nil {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	results, err := h.svc.Publish(r.Context(), u.ID, id, req.Force)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"results": results, "judging_status": "published"})
}

func (h *Handler) Results(w http.ResponseWriter, r *http.Request) {
	id, ok := eventID(w, r)
	if !ok {
		return
	}
	var viewer *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		viewer = &u.ID
	}
	results, err := h.svc.Results(r.Context(), viewer, id)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"results": results})
}

func eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return uuid.Nil, false
	}
	return id, true
}
