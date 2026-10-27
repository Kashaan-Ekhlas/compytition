// Package support groups the smaller account surfaces: notifications,
// feedback reports and event bookmarks. All queries are scoped to the calling
// user; only admins can read the feedback queue.
package support

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
)

type Service struct {
	store *db.Store
	cfg   *config.Config
	log   *slog.Logger
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{store: store, cfg: cfg, log: log}
}

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Link      string     `json:"link"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type Bookmark struct {
	EventID      uuid.UUID  `json:"event_id"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	StartsAt     *time.Time `json:"starts_at,omitempty"`
	BookmarkedAt time.Time  `json:"bookmarked_at"`
}

type Feedback struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id"`
	Username  string     `json:"username,omitempty"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Service) Notifications(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int32) ([]Notification, error) {
	var unread *bool
	if unreadOnly {
		value := true
		unread = &value
	}
	rows, err := s.store.ListNotifications(ctx, db.ListNotificationsParams{
		UserID: userID, UnreadOnly: unread, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, Notification{
			ID: r.ID, Kind: r.Kind, Title: r.Title, Body: r.Body, Link: r.Link,
			ReadAt: r.ReadAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int32, error) {
	return s.store.CountUnreadNotifications(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	return s.store.MarkNotificationRead(ctx, db.MarkNotificationReadParams{ID: notificationID, UserID: userID})
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.store.MarkAllNotificationsRead(ctx, userID)
}

func (s *Service) CreateFeedback(ctx context.Context, userID *uuid.UUID, subject, body string) (*Feedback, error) {
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	fields := map[string]string{}
	if len(subject) < 3 || len(subject) > 160 {
		fields["subject"] = "Subject must be 3–160 characters."
	}
	if len(body) < 3 || len(body) > 5000 {
		fields["body"] = "Message must be 3–5,000 characters."
	}
	if len(fields) > 0 {
		return nil, apperr.InvalidFields(fields)
	}
	row, err := s.store.CreateFeedback(ctx, db.CreateFeedbackParams{
		UserID: userID, Subject: subject, Body: body,
	})
	if err != nil {
		return nil, err
	}
	return &Feedback{
		ID: row.ID, UserID: row.UserID, Subject: row.Subject, Body: row.Body,
		Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Service) ListFeedback(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]Feedback, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if !user.IsAdmin && !s.cfg.IsAdminEmail(user.Email) {
		return nil, apperr.ErrNotFound
	}
	rows, err := s.store.ListFeedback(ctx, db.ListFeedbackParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]Feedback, 0, len(rows))
	for _, r := range rows {
		out = append(out, Feedback{
			ID: r.ID, UserID: r.UserID, Username: deref(r.Username),
			Subject: r.Subject, Body: r.Body, Status: r.Status, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) Bookmarks(ctx context.Context, userID uuid.UUID) ([]Bookmark, error) {
	rows, err := s.store.ListBookmarkedEvents(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Bookmark, 0, len(rows))
	for _, r := range rows {
		out = append(out, Bookmark{
			EventID: r.ID, Slug: r.Slug, Title: r.Title, Status: r.Status,
			StartsAt: r.StartsAt, BookmarkedAt: r.BookmarkedAt,
		})
	}
	return out, nil
}

func (s *Service) AddBookmark(ctx context.Context, userID, eventID uuid.UUID) error {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.Status == "draft" {
		return apperr.ErrNotFound
	}
	return s.store.AddBookmark(ctx, db.AddBookmarkParams{UserID: userID, EventID: eventID})
}

func (s *Service) RemoveBookmark(ctx context.Context, userID, eventID uuid.UUID) error {
	return s.store.RemoveBookmark(ctx, db.RemoveBookmarkParams{UserID: userID, EventID: eventID})
}

func (s *Service) IsBookmarked(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	return s.store.IsBookmarked(ctx, db.IsBookmarkedParams{UserID: userID, EventID: eventID})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
