package support

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

func (h *Handler) Notifications(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	limit, offset := httpx.Pagination(r, 30, 100)
	unread := r.URL.Query().Get("unread") == "1"
	list, err := h.svc.Notifications(r.Context(), u.ID, unread, limit, offset)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	count, err := h.svc.UnreadCount(r.Context(), u.ID)
	if err != nil {
		h.log.Warn("unread notification count", "error", err)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"notifications": list, "unread": count, "limit": limit, "offset": offset,
	})
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Notification not found.")
		return
	}
	if err := h.svc.MarkRead(r.Context(), u.ID, id); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	if err := h.svc.MarkAllRead(r.Context(), u.ID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Feedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 64<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	var userID *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		userID = &u.ID
	}
	report, err := h.svc.CreateFeedback(r.Context(), userID, req.Subject, req.Body)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"feedback": report})
}

func (h *Handler) AdminFeedback(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	limit, offset := httpx.Pagination(r, 50, 200)
	list, err := h.svc.ListFeedback(r.Context(), u.ID, limit, offset)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"feedback": list, "limit": limit, "offset": offset})
}

func (h *Handler) Bookmarks(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.Bookmarks(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"bookmarks": list})
}

func (h *Handler) AddBookmark(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	if err := h.svc.AddBookmark(r.Context(), u.ID, eventID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"bookmarked": true})
}

func (h *Handler) RemoveBookmark(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	if err := h.svc.RemoveBookmark(r.Context(), u.ID, eventID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}
