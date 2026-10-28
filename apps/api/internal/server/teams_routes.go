package server

import (
	"net/http"

	"compytition/internal/teams"
)

func registerTeamRoutes(mux *http.ServeMux, api *API) {
	svc := teams.NewService(api.Store, api.Cfg, api.Log)
	h := teams.NewHandler(svc, api.Log)

	mux.Handle("POST /api/v1/events/{id}/teams", RequireAuth(http.HandlerFunc(h.Create)))
	mux.HandleFunc("GET /api/v1/events/{id}/teams", h.ListForEvent)
	mux.Handle("GET /api/v1/events/{id}/teams/mine", RequireAuth(http.HandlerFunc(h.MineForEvent)))
	mux.Handle("GET /api/v1/teams", RequireAuth(http.HandlerFunc(h.ListMine)))
	mux.HandleFunc("GET /api/v1/teams/{id}", h.Get)
	mux.Handle("PATCH /api/v1/teams/{id}", RequireAuth(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/teams/{id}", RequireAuth(http.HandlerFunc(h.Dissolve)))
	mux.Handle("POST /api/v1/teams/{id}/leave", RequireAuth(http.HandlerFunc(h.Leave)))
	mux.Handle("POST /api/v1/teams/{id}/invites", RequireAuth(http.HandlerFunc(h.Invite)))
	mux.Handle("GET /api/v1/teams/{id}/invite-link", RequireAuth(http.HandlerFunc(h.InviteLink)))
	mux.HandleFunc("GET /api/v1/team-invites/preview", h.PreviewInviteLink)
	mux.Handle("POST /api/v1/team-invites/accept-link", RequireAuth(http.HandlerFunc(h.AcceptInviteLink)))
	mux.Handle("GET /api/v1/teams/{id}/invites", RequireAuth(http.HandlerFunc(h.ListInvites)))
	mux.Handle("DELETE /api/v1/teams/{id}/invites/{inviteId}", RequireAuth(http.HandlerFunc(h.CancelInvite)))
	mux.Handle("POST /api/v1/team-invites/{id}/accept", RequireAuth(h.RespondInvite(true)))
	mux.Handle("POST /api/v1/team-invites/{id}/reject", RequireAuth(h.RespondInvite(false)))
	mux.Handle("GET /api/v1/team-invites", RequireAuth(http.HandlerFunc(h.MyInvites)))
	mux.Handle("POST /api/v1/teams/{id}/join-requests", RequireAuth(http.HandlerFunc(h.RequestToJoin)))
	mux.Handle("GET /api/v1/teams/{id}/join-requests", RequireAuth(http.HandlerFunc(h.ListJoinRequests)))
	mux.Handle("POST /api/v1/team-join-requests/{id}/accept", RequireAuth(h.RespondJoinRequest(true)))
	mux.Handle("POST /api/v1/team-join-requests/{id}/reject", RequireAuth(h.RespondJoinRequest(false)))
	mux.Handle("GET /api/v1/team-join-requests", RequireAuth(http.HandlerFunc(h.MyJoinRequests)))
}
