package submissions

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
	eventID, ok := pathID(w, r)
	if !ok {
		return
	}
	var in Input
	if err := httpx.DecodeJSON(w, r, &in, 64<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	sub, err := h.svc.Create(r.Context(), u.ID, eventID, in)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"submission": sub})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in Input
	if err := httpx.DecodeJSON(w, r, &in, 64<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	sub, err := h.svc.Update(r.Context(), u.ID, id, in)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"submission": sub})
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r)
	if !ok {
		return
	}
	sub, err := h.svc.GetMine(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"submission": sub})
}

func (h *Handler) ListForEvent(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := pathID(w, r)
	if !ok {
		return
	}
	limit, offset := httpx.Pagination(r, 50, 200)
	list, total, err := h.svc.ListForEvent(r.Context(), u.ID, eventID, r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"submissions": list, "total": total, "limit": limit, "offset": offset,
	})
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListMine(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"submissions": list})
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Not found.")
		return uuid.Nil, false
	}
	return id, true
}
