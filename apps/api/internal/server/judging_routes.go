package server

import (
	"net/http"

	"compytition/internal/judging"
)

func registerJudgingRoutes(mux *http.ServeMux, api *API) {
	h := judging.NewHandler(judging.NewService(api.Store, api.Cfg, api.Log), api.Log)

	mux.Handle("GET /api/v1/events/{id}/judging/preview", RequireAuth(http.HandlerFunc(h.Preview)))
	mux.Handle("POST /api/v1/events/{id}/judging/assign", RequireAuth(http.HandlerFunc(h.Assign)))
	mux.Handle("POST /api/v1/events/{id}/judging/start", RequireAuth(http.HandlerFunc(h.Start)))
	mux.Handle("GET /api/v1/events/{id}/judging/review", RequireAuth(http.HandlerFunc(h.Review)))
	mux.Handle("POST /api/v1/events/{id}/judging/publish", RequireAuth(http.HandlerFunc(h.Publish)))
	mux.HandleFunc("GET /api/v1/events/{id}/results", h.Results)

	mux.Handle("GET /api/v1/judge/events/{id}/next", RequireAuth(http.HandlerFunc(h.Next)))
	mux.Handle("GET /api/v1/judge/events/{id}/progress", RequireAuth(http.HandlerFunc(h.Progress)))
	mux.Handle("POST /api/v1/judge/events/{id}/comparisons/{pairId}", RequireAuth(http.HandlerFunc(h.Compare)))
	mux.Handle("POST /api/v1/judge/events/{id}/comparisons/{pairId}/undo", RequireAuth(http.HandlerFunc(h.Undo)))
}
