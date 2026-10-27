// Package chat provides team-scoped chat: persisted history over HTTP and a
// websocket fan-out per team. Membership is re-checked on every inbound
// websocket message, so losing access takes effect immediately.
package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 4096
	maxBodyLength  = 2000
	sendBuffer     = 32
)

type Service struct {
	store *db.Store
	cfg   *config.Config
	log   *slog.Logger
	hub   *hub
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{store: store, cfg: cfg, log: log, hub: newHub()}
}

type Message struct {
	ID        uuid.UUID `json:"id"`
	TeamID    uuid.UUID `json:"team_id"`
	UserID    uuid.UUID `json:"user_id"`
	Body      string    `json:"body"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) History(ctx context.Context, userID, teamID uuid.UUID, before *time.Time, limit int32) ([]Message, error) {
	if err := s.requireMember(ctx, userID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListTeamMessages(ctx, db.ListTeamMessagesParams{
		TeamID: teamID, Before: before, Limit: limit, Offset: 0,
	})
	if err != nil {
		return nil, err
	}
	// The query returns newest first for cheap pagination; the UI wants oldest
	// first inside a page.
	out := make([]Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		out = append(out, Message{
			ID: r.ID, TeamID: r.TeamID, UserID: r.UserID, Body: r.Body,
			Username: r.Username, FullName: deref(r.FullName), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) requireMember(ctx context.Context, userID, teamID uuid.UUID) error {
	_, err := s.store.GetTeamMember(ctx, db.GetTeamMemberParams{TeamID: teamID, UserID: userID})
	if err != nil {
		return apperr.ErrNotFound
	}
	return nil
}

// ServeWS upgrades the connection and runs the read/write pumps.
func (s *Service) ServeWS(w http.ResponseWriter, r *http.Request, userID, teamID uuid.UUID) {
	if err := s.requireMember(r.Context(), userID, teamID); err != nil {
		http.Error(w, "Team not found.", http.StatusNotFound)
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Non-browser clients are allowed; browsers always send Origin.
				return true
			}
			return s.originAllowed(origin, r.Host)
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("websocket upgrade failed", "error", err)
		return
	}
	c := &client{
		svc: s, conn: conn, teamID: teamID, userID: userID,
		send: make(chan []byte, sendBuffer), done: make(chan struct{}),
	}
	s.hub.add(c)
	go c.writePump()
	c.readPump()
}

func (s *Service) originAllowed(origin, host string) bool {
	if strings.EqualFold(strings.TrimRight(origin, "/"), s.cfg.WebOrigin) {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

func (s *Service) saveMessage(ctx context.Context, userID, teamID uuid.UUID, body string) (*Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > maxBodyLength {
		return nil, apperr.Invalid("body", "Message must be 1–2,000 characters.")
	}
	// Re-check on every message: a removed member's socket must stop working.
	if err := s.requireMember(ctx, userID, teamID); err != nil {
		return nil, err
	}
	row, err := s.store.CreateChatMessage(ctx, db.CreateChatMessageParams{
		TeamID: teamID, UserID: userID, Body: body,
	})
	if err != nil {
		return nil, err
	}
	full, err := s.store.GetChatMessage(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return &Message{
		ID: full.ID, TeamID: full.TeamID, UserID: full.UserID, Body: full.Body,
		Username: full.Username, FullName: deref(full.FullName), CreatedAt: full.CreatedAt,
	}, nil
}

// ── Hub ─────────────────────────────────────────────────────────────────────

type hub struct {
	mu    sync.RWMutex
	teams map[uuid.UUID]map[*client]struct{}
}

func newHub() *hub { return &hub{teams: map[uuid.UUID]map[*client]struct{}{}} }

func (h *hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.teams[c.teamID]
	if set == nil {
		set = map[*client]struct{}{}
		h.teams[c.teamID] = set
	}
	set[c] = struct{}{}
}

func (h *hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.teams[c.teamID]
	if set == nil {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.teams, c.teamID)
	}
}

func (h *hub) broadcast(teamID uuid.UUID, payload []byte) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.teams[teamID]))
	for c := range h.teams[teamID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.send <- payload:
		default:
			// Slow consumer: drop the frame rather than block the team.
			c.close()
		}
	}
}

// ── Client ──────────────────────────────────────────────────────────────────

type client struct {
	svc    *Service
	conn   *websocket.Conn
	teamID uuid.UUID
	userID uuid.UUID

	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.svc.hub.remove(c)
		_ = c.conn.Close()
	})
}

type inbound struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func (c *client) readPump() {
	defer c.close()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	windowStart := time.Now()
	count := 0
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if time.Since(windowStart) > 5*time.Second {
			windowStart = time.Now()
			count = 0
		}
		count++
		if count > 20 {
			c.writeError("You're sending messages too quickly.")
			return
		}

		var msg inbound
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.writeError("Invalid message.")
			continue
		}
		if msg.Type == "ping" {
			c.enqueue(map[string]any{"type": "pong"})
			continue
		}
		saved, err := c.svc.saveMessage(context.Background(), c.userID, c.teamID, msg.Body)
		if err != nil {
			if err == apperr.ErrNotFound {
				c.writeError("You are no longer a member of this team.")
				return
			}
			c.writeError(err.Error())
			continue
		}
		payload, err := json.Marshal(map[string]any{"type": "message", "message": saved})
		if err != nil {
			continue
		}
		c.svc.hub.broadcast(c.teamID, payload)
	}
}

func (c *client) enqueue(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	case <-c.done:
	}
}

func (c *client) writeError(message string) {
	c.enqueue(map[string]any{"type": "error", "message": message})
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.close()
	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
			return
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
