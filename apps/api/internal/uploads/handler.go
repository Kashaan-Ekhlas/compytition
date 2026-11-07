package uploads

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"os"
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	kind := r.URL.Query().Get("kind")
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes(kind))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "That file is too large.")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "Send the file as multipart form data.")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	var eventID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("event_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpx.FieldError(w, map[string]string{"event_id": "Choose a valid event."})
			return
		}
		eventID = &parsed
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		httpx.FieldError(w, map[string]string{"file": "Choose a file to upload."})
		return
	}
	defer src.Close()

	view, err := h.svc.Save(r.Context(), u.ID, kind, eventID, header, src)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"upload": view})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "File not found.")
		return
	}
	var userID *uuid.UUID
	isAdmin := false
	if u, ok := httpx.UserFrom(r.Context()); ok {
		userID = &u.ID
		isAdmin = u.IsAdmin
	}
	preview := r.URL.Query().Get("preview") == "1"
	download := r.URL.Query().Get("download") == "1"
	file, err := h.svc.Get(r.Context(), userID, isAdmin, id, preview)
	if err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	f, err := os.Open(file.Path)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "File not found.")
		return
	}
	defer f.Close()

	disposition := "inline"
	if download || (file.Kind == KindAttachment && file.Mime != "application/pdf") {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", file.Mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Uploads are presented in an iframe on the web origin. The global API
	// headers deny framing for JSON endpoints, so relax them here for files.
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Name}))
	if file.Kind == KindAttachment {
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, file.Name, file.ModTime, f)
}

func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := httpx.MustUser(r.Context())
	if err := h.svc.DeleteAvatar(r.Context(), u.ID); err != nil {
		httpx.WriteServiceError(w, h.log, err)
		return
	}
	httpx.NoContent(w)
}
