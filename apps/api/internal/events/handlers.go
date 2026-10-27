package events

import (
	"log/slog"
	"net/http"
	"strings"

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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := EventFilters{
		Search: q.Get("q"), Category: q.Get("category"), Format: q.Get("format"),
		RegStatus: q.Get("status"),
	}
	f.Limit, f.Offset = httpx.Pagination(r, 24, 100)
	list, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"events": list, "total": total, "limit": f.Limit, "offset": f.Offset,
		"categories": SortedCategories(),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	var viewer *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		viewer = &u.ID
	}
	ev, err := h.svc.Get(r.Context(), viewer, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"event": ev})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req EventFields
	if err := httpx.DecodeJSON(w, r, &req, 64<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	ev, err := h.svc.Create(r.Context(), u.ID, req)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"event": ev})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	var req EventFields
	if err := httpx.DecodeJSON(w, r, &req, 64<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	ev, err := h.svc.Update(r.Context(), u.ID, eventID, req)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"event": ev})
}

func (h *Handler) SetStatus(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := httpx.MustUser(r.Context())
		eventID, err := httpx.PathUUID(r, "id")
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "Event not found.")
			return
		}
		if err := h.svc.SetStatus(r.Context(), u.ID, eventID, target); err != nil {
			httpx.WriteServiceError(w, h.log, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": target})
	}
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	if err := h.svc.Delete(r.Context(), u.ID, eventID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Manage(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	ev, err := h.svc.GetManage(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"event": ev})
}

func (h *Handler) Hosted(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListHosted(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"events": list})
}

func (h *Handler) JudgeEvents(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListForJudge(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"events": list})
}

func (h *Handler) ReplaceForm(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	var req struct {
		Fields []FormFieldInput `json:"fields"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 128<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	fields, err := h.svc.ReplaceForm(r.Context(), u.ID, eventID, req.Fields)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"fields": fields})
}

func (h *Handler) Form(w http.ResponseWriter, r *http.Request) {
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	fields, err := h.svc.FormFields(r.Context(), eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"fields": fields})
}

func (h *Handler) InviteStaff(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	staff, err := h.svc.InviteStaff(r.Context(), u.ID, eventID, req.Email, req.Role)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"staff": staff})
}

func (h *Handler) ListStaff(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "judge"
	}
	if role != "judge" && role != "co_organizer" {
		httpx.Error(w, http.StatusBadRequest, "Unknown role.")
		return
	}
	staff, err := h.svc.ListStaff(r.Context(), u.ID, eventID, role)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"staff": staff})
}

func (h *Handler) AcceptStaffInvite(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		httpx.Error(w, http.StatusNotFound, "Invite not found.")
		return
	}
	eventID, role, err := h.svc.AcceptStaffInvite(r.Context(), u.ID, token)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"event_id": eventID, "role": role})
}

func (h *Handler) RemoveStaff(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	staffID, err := httpx.PathUUID(r, "staffId")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Staff member not found.")
		return
	}
	if err := h.svc.RemoveStaff(r.Context(), u.ID, eventID, staffID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) GetJudgingConfig(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	cfg, criteria, err := h.svc.GetJudgingConfig(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"config": cfg, "criteria": criteria})
}

func (h *Handler) SetJudgingConfig(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return
	}
	var req JudgingConfig
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := h.svc.SetJudgingConfig(r.Context(), u.ID, eventID, req)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"config": cfg})
}
