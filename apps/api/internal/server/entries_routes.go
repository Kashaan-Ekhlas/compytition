package server

import (
	"net/http"

	"compytition/internal/registrations"
	"compytition/internal/submissions"
)

func registerRegistrationRoutes(mux *http.ServeMux, api *API) {
	h := registrations.NewHandler(registrations.NewService(api.Store, api.Cfg, api.Log), api.Log)

	mux.Handle("POST /api/v1/events/{id}/registrations", RequireAuth(http.HandlerFunc(h.Create)))
	mux.Handle("GET /api/v1/events/{id}/registrations/me", RequireAuth(http.HandlerFunc(h.Mine)))
	mux.Handle("PATCH /api/v1/events/{id}/registrations/me", RequireAuth(http.HandlerFunc(h.UpdateAnswers)))
	mux.Handle("DELETE /api/v1/events/{id}/registrations/me", RequireAuth(http.HandlerFunc(h.Cancel)))
	mux.Handle("GET /api/v1/events/{id}/registrations", RequireAuth(http.HandlerFunc(h.ListForEvent)))
	mux.Handle("PATCH /api/v1/registrations/{id}/status", RequireAuth(http.HandlerFunc(h.SetStatus)))
	mux.Handle("GET /api/v1/registrations", RequireAuth(http.HandlerFunc(h.ListMine)))
}

func registerSubmissionRoutes(mux *http.ServeMux, api *API) {
	h := submissions.NewHandler(submissions.NewService(api.Store, api.Cfg, api.Log), api.Log)

	mux.Handle("GET /api/v1/events/{id}/submissions/me", RequireAuth(http.HandlerFunc(h.Mine)))
	mux.Handle("POST /api/v1/events/{id}/submissions", RequireAuth(http.HandlerFunc(h.Create)))
	mux.Handle("PATCH /api/v1/submissions/{id}", RequireAuth(http.HandlerFunc(h.Update)))
	mux.Handle("GET /api/v1/events/{id}/submissions", RequireAuth(http.HandlerFunc(h.ListForEvent)))
	mux.Handle("GET /api/v1/submissions", RequireAuth(http.HandlerFunc(h.ListMine)))
}
