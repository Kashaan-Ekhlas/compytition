package server

import (
	"net/http"

	"compytition/internal/events"
)

func registerEventRoutes(mux *http.ServeMux, api *API) {
	svc := events.NewService(api.Store, api.Cfg, api.Log)
	h := events.NewHandler(svc, api.Log)

	mux.HandleFunc("GET /api/v1/events", h.List)
	mux.HandleFunc("GET /api/v1/events/{id}", h.Get)
	mux.HandleFunc("GET /api/v1/events/{id}/form", h.Form)
	mux.Handle("POST /api/v1/events", RequireAuth(http.HandlerFunc(h.Create)))
	mux.Handle("PATCH /api/v1/events/{id}", RequireAuth(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/events/{id}", RequireAuth(http.HandlerFunc(h.Delete)))
	mux.Handle("POST /api/v1/events/{id}/publish", RequireAuth(h.SetStatus("published")))
	mux.Handle("POST /api/v1/events/{id}/unpublish", RequireAuth(h.SetStatus("draft")))
	mux.Handle("POST /api/v1/events/{id}/close", RequireAuth(h.SetStatus("closed")))
	mux.Handle("GET /api/v1/events/{id}/manage", RequireAuth(http.HandlerFunc(h.Manage)))
	mux.Handle("GET /api/v1/host/events", RequireAuth(http.HandlerFunc(h.Hosted)))
	mux.Handle("GET /api/v1/judge/events", RequireAuth(http.HandlerFunc(h.JudgeEvents)))
	mux.Handle("PUT /api/v1/events/{id}/form", RequireAuth(http.HandlerFunc(h.ReplaceForm)))
	mux.Handle("GET /api/v1/events/{id}/staff", RequireAuth(http.HandlerFunc(h.ListStaff)))
	mux.Handle("POST /api/v1/events/{id}/staff", RequireAuth(http.HandlerFunc(h.InviteStaff)))
	mux.Handle("DELETE /api/v1/events/{id}/staff/{staffId}", RequireAuth(http.HandlerFunc(h.RemoveStaff)))
	mux.Handle("POST /api/v1/invites/staff/{token}/accept", RequireAuth(http.HandlerFunc(h.AcceptStaffInvite)))
	mux.Handle("GET /api/v1/events/{id}/judging-config", RequireAuth(http.HandlerFunc(h.GetJudgingConfig)))
	mux.Handle("PUT /api/v1/events/{id}/judging-config", RequireAuth(http.HandlerFunc(h.SetJudgingConfig)))
}
