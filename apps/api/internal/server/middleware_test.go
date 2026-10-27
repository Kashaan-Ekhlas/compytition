package server

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hijackRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestLoggingPreservesHijacker(t *testing.T) {
	rec := &hijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("logging middleware hid http.Hijacker")
		}
		if _, _, err := hj.Hijack(); err != nil {
			t.Fatalf("hijack: %v", err)
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/teams/1/chat", nil))
	if !rec.hijacked {
		t.Fatal("underlying writer was not hijacked")
	}
}

func TestLoggingExposesResponseController(t *testing.T) {
	rec := httptest.NewRecorder()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("flush through response controller: %v", err)
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
}
