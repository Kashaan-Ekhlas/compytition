package server

import (
	"net/http"

	"compytition/internal/admin"
)

func registerAdminRoutes(mux *http.ServeMux, api *API) {
	h := admin.NewHandler(admin.NewService(api.Store, api.Cfg, api.Log), api.Log)

	mux.Handle("GET /api/v1/admin/overview", RequireAuth(http.HandlerFunc(h.Overview)))
	// Seeding creates demo accounts with a public password, so it is only
	// available outside production.
	if api.Cfg.Env != "production" {
		mux.Handle("POST /api/v1/admin/seed", RequireAuth(http.HandlerFunc(h.Seed)))
		mux.Handle("POST /api/v1/admin/reset", RequireAuth(http.HandlerFunc(h.Reset)))
	}
}
