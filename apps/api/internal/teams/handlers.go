package teams

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
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.svc.Create(r.Context(), u.ID, eventID, req.Name)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"team": team})
}

func (h *Handler) ListForEvent(w http.ResponseWriter, r *http.Request) {
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	var viewer *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		viewer = &u.ID
	}
	list, err := h.svc.ListForEvent(r.Context(), viewer, eventID, ListOptions{
		LookingForMembers: r.URL.Query().Get("looking") == "1",
	})
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teams": list})
}

func (h *Handler) MineForEvent(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	team, err := h.svc.GetForEventUser(r.Context(), u.ID, eventID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"team": team})
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListMine(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teams": list})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	var viewer *uuid.UUID
	if u, ok := httpx.UserFrom(r.Context()); ok {
		viewer = &u.ID
	}
	team, err := h.svc.Get(r.Context(), viewer, teamID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"team": team})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name                string `json:"name"`
		IsLookingForMembers bool   `json:"is_looking_for_members"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Update(r.Context(), u.ID, teamID, req.Name, req.IsLookingForMembers); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	team, err := h.svc.Get(r.Context(), &u.ID, teamID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"team": team})
}

func (h *Handler) Dissolve(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Dissolve(r.Context(), u.ID, teamID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Leave(r.Context(), u.ID, teamID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	inv, err := h.svc.Invite(r.Context(), u.ID, teamID, req.Username)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"invite": inv})
}

func (h *Handler) ListInvites(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListInvites(r.Context(), u.ID, teamID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invites": list})
}

func (h *Handler) CancelInvite(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	inviteID, err := httpx.PathUUID(r, "inviteId")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Invite not found.")
		return
	}
	if err := h.svc.CancelInvite(r.Context(), u.ID, teamID, inviteID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) RespondInvite(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := httpx.MustUser(r.Context())
		inviteID, err := httpx.PathUUID(r, "id")
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "Invite not found.")
			return
		}
		team, err := h.svc.RespondInvite(r.Context(), u.ID, inviteID, accept)
		if err != nil {
			httpx.WriteServiceError(w, h.log, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"accepted": accept, "team": team})
	}
}

func (h *Handler) MyInvites(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListMyInvites(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invites": list})
}

func (h *Handler) RequestToJoin(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	jr, err := h.svc.RequestToJoin(r.Context(), u.ID, teamID, req.Message)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"request": jr})
}

func (h *Handler) ListJoinRequests(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListJoinRequests(r.Context(), u.ID, teamID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"requests": list})
}

func (h *Handler) RespondJoinRequest(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := httpx.MustUser(r.Context())
		reqID, err := httpx.PathUUID(r, "id")
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "Request not found.")
			return
		}
		team, err := h.svc.RespondJoinRequest(r.Context(), u.ID, reqID, accept)
		if err != nil {
			httpx.WriteServiceError(w, h.log, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"accepted": accept, "team": team})
	}
}

func (h *Handler) MyJoinRequests(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	list, err := h.svc.ListMyJoinRequests(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"requests": list})
}

func (h *Handler) eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Event not found.")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) teamID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Team not found.")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) InviteLink(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	teamID, ok := h.teamID(w, r)
	if !ok {
		return
	}
	link, err := h.svc.InviteLink(r.Context(), u.ID, teamID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invite_link": link})
}

func (h *Handler) PreviewInviteLink(w http.ResponseWriter, r *http.Request) {
	link, err := h.svc.PreviewInviteLink(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invite_link": link})
}

func (h *Handler) AcceptInviteLink(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	var req struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(w, r, &req, 8<<10); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.svc.AcceptInviteLink(r.Context(), u.ID, req.Token)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"team": team})
}
