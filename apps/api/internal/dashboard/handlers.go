package dashboard

import (
	"log/slog"
	"net/http"

	"compytition/internal/httpx"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	res, err := h.svc.Get(r.Context(), u.ID)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
