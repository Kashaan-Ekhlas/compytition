package server

import (
	"net/http"

	"compytition/internal/chat"
	"compytition/internal/dashboard"
	"compytition/internal/support"
)

func registerDashboardRoutes(mux *http.ServeMux, api *API) {
	h := dashboard.NewHandler(dashboard.NewService(api.Store, api.Cfg, api.Log), api.Log)
	mux.Handle("GET /api/v1/dashboard", RequireAuth(http.HandlerFunc(h.Get)))
}

func registerChatRoutes(mux *http.ServeMux, api *API) {
	h := chat.NewHandler(chat.NewService(api.Store, api.Cfg, api.Log), api.Log)
	mux.Handle("GET /api/v1/teams/{id}/messages", RequireAuth(http.HandlerFunc(h.History)))
	mux.Handle("GET /api/v1/teams/{id}/chat", RequireAuth(http.HandlerFunc(h.Connect)))
}

func registerSupportRoutes(mux *http.ServeMux, api *API) {
	h := support.NewHandler(support.NewService(api.Store, api.Cfg, api.Log), api.Log)

	mux.Handle("GET /api/v1/notifications", RequireAuth(http.HandlerFunc(h.Notifications)))
	mux.Handle("POST /api/v1/notifications/{id}/read", RequireAuth(http.HandlerFunc(h.MarkRead)))
	mux.Handle("POST /api/v1/notifications/read-all", RequireAuth(http.HandlerFunc(h.MarkAllRead)))
	mux.HandleFunc("POST /api/v1/feedback", h.Feedback)
	mux.Handle("GET /api/v1/admin/feedback", RequireAuth(http.HandlerFunc(h.AdminFeedback)))
	mux.Handle("GET /api/v1/bookmarks", RequireAuth(http.HandlerFunc(h.Bookmarks)))
	mux.Handle("POST /api/v1/events/{id}/bookmark", RequireAuth(http.HandlerFunc(h.AddBookmark)))
	mux.Handle("DELETE /api/v1/events/{id}/bookmark", RequireAuth(http.HandlerFunc(h.RemoveBookmark)))
}
