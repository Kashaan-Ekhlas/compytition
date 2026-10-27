package submissions

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

type Input struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	RepoURL     string `json:"repo_url"`
	DemoURL     string `json:"demo_url"`
	FileURL     string `json:"file_url"`
	Status      string `json:"status"`
}

type View struct {
	ID                 uuid.UUID  `json:"id"`
	EventID            uuid.UUID  `json:"event_id"`
	EventTitle         string     `json:"event_title,omitempty"`
	EventSlug          string     `json:"event_slug,omitempty"`
	SubmissionDeadline *time.Time `json:"submission_deadline,omitempty"`
	RegistrationID     *uuid.UUID `json:"registration_id"`
	TeamID             *uuid.UUID `json:"team_id"`
	TeamName           string     `json:"team_name,omitempty"`
	CreatedBy          uuid.UUID  `json:"created_by"`
	CreatorUsername    string     `json:"creator_username,omitempty"`
	CreatorName        string     `json:"creator_name,omitempty"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	RepoURL            string     `json:"repo_url"`
	DemoURL            string     `json:"demo_url"`
	FileURL            string     `json:"file_url"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (s *Service) Create(ctx context.Context, userID, eventID uuid.UUID, in Input) (*View, error) {
	if err := validateInput(&in); err != nil {
		return nil, err
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := submissionOpen(ev); err != nil {
		return nil, err
	}
	reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil || reg.Status != "registered" {
		return nil, apperr.Invalid("registration", "Register for this event before submitting.")
	}
	if reg.TeamID != nil {
		if _, err := s.store.GetSubmissionByTeam(ctx, db.GetSubmissionByTeamParams{EventID: eventID, TeamID: reg.TeamID}); err == nil {
			return nil, apperr.Invalid("submission", "Your team already has a submission. Edit it instead.")
		}
	} else {
		if _, err := s.store.GetSubmissionByRegistration(ctx, &reg.ID); err == nil {
			return nil, apperr.Invalid("submission", "You already submitted. Edit your submission instead.")
		}
	}
	sub, err := s.store.CreateSubmission(ctx, db.CreateSubmissionParams{
		EventID: eventID, RegistrationID: &reg.ID, TeamID: reg.TeamID, CreatedBy: userID,
		Title: in.Title, Description: in.Description, RepoUrl: in.RepoURL,
		DemoUrl: in.DemoURL, FileUrl: in.FileURL, Status: normalizedStatus(in.Status),
	})
	if err != nil {
		if isUnique(err) {
			return nil, apperr.Invalid("submission", "A submission already exists for this entry.")
		}
		return nil, err
	}
	v := view(sub)
	v.EventTitle = ev.Title
	v.EventSlug = ev.Slug
	v.SubmissionDeadline = ev.SubmissionDeadline
	if reg.TeamID != nil {
		if team, err := s.store.GetTeamByID(ctx, *reg.TeamID); err == nil {
			v.TeamName = team.Name
		}
	}
	if u, err := s.store.GetUserByID(ctx, userID); err == nil {
		v.CreatorUsername = u.Username
	}
	return &v, nil
}

func (s *Service) Update(ctx context.Context, userID, submissionID uuid.UUID, in Input) (*View, error) {
	if err := validateInput(&in); err != nil {
		return nil, err
	}
	sub, err := s.store.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, sub.EventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := s.canEdit(ctx, userID, sub); err != nil {
		return nil, err
	}
	if err := submissionOpen(ev); err != nil {
		return nil, err
	}
	updated, err := s.store.UpdateSubmission(ctx, db.UpdateSubmissionParams{
		ID: submissionID, Title: in.Title, Description: in.Description,
		RepoUrl: in.RepoURL, DemoUrl: in.DemoURL, FileUrl: in.FileURL,
		Status: normalizedStatus(in.Status),
	})
	if err != nil {
		return nil, err
	}
	v := view(updated)
	v.EventTitle = ev.Title
	v.EventSlug = ev.Slug
	v.SubmissionDeadline = ev.SubmissionDeadline
	if updated.TeamID != nil {
		if team, err := s.store.GetTeamByID(ctx, *updated.TeamID); err == nil {
			v.TeamName = team.Name
		}
	}
	return &v, nil
}

func (s *Service) GetMine(ctx context.Context, userID, eventID uuid.UUID) (*View, error) {
	sub, err := s.store.GetSubmissionForUser(ctx, db.GetSubmissionForUserParams{EventID: eventID, CreatedBy: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	v := view(sub)
	v.EventTitle = ev.Title
	v.EventSlug = ev.Slug
	v.SubmissionDeadline = ev.SubmissionDeadline
	if sub.TeamID != nil {
		if team, err := s.store.GetTeamByID(ctx, *sub.TeamID); err == nil {
			v.TeamName = team.Name
		}
	}
	return &v, nil
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]View, error) {
	rows, err := s.store.ListMySubmissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		out = append(out, View{
			ID: r.ID, EventID: r.EventID, EventTitle: r.EventTitle, EventSlug: r.EventSlug,
			SubmissionDeadline: r.SubmissionDeadline, RegistrationID: r.RegistrationID,
			TeamID: r.TeamID, TeamName: deref(r.TeamName), CreatedBy: r.CreatedBy,
			Title: r.Title, Description: r.Description, RepoURL: r.RepoUrl,
			DemoURL: r.DemoUrl, FileURL: r.FileUrl, Status: r.Status,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) ListForEvent(ctx context.Context, userID, eventID uuid.UUID, status string, limit, offset int32) ([]View, int64, error) {
	allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: eventID})
	if err != nil {
		return nil, 0, err
	}
	if !allowed {
		return nil, 0, apperr.ErrNotFound
	}
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	rows, err := s.store.ListSubmissionsForEvent(ctx, db.ListSubmissionsForEventParams{
		EventID: eventID, Status: statusPtr, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]View, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		out = append(out, View{
			ID: r.ID, EventID: r.EventID, RegistrationID: r.RegistrationID, TeamID: r.TeamID,
			TeamName: deref(r.TeamName), CreatedBy: r.CreatedBy, CreatorUsername: r.CreatorUsername,
			CreatorName: deref(r.CreatorName), Title: r.Title, Description: r.Description,
			RepoURL: r.RepoUrl, DemoURL: r.DemoUrl, FileURL: r.FileUrl, Status: r.Status,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, total, nil
}

func (s *Service) canEdit(ctx context.Context, userID uuid.UUID, sub db.Submission) error {
	if sub.CreatedBy == userID {
		return nil
	}
	if sub.TeamID != nil {
		if _, err := s.store.GetTeamMember(ctx, db.GetTeamMemberParams{TeamID: *sub.TeamID, UserID: userID}); err == nil {
			return nil
		}
	}
	return apperr.ErrNotFound
}

func validateInput(in *Input) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.RepoURL = strings.TrimSpace(in.RepoURL)
	in.DemoURL = strings.TrimSpace(in.DemoURL)
	in.FileURL = strings.TrimSpace(in.FileURL)
	fields := map[string]string{}
	if len(in.Title) < 1 || len(in.Title) > 160 {
		fields["title"] = "Title must be 1–160 characters."
	}
	if len(in.Description) > 20_000 {
		fields["description"] = "Keep the description under 20,000 characters."
	}
	for key, val := range map[string]string{"repo_url": in.RepoURL, "demo_url": in.DemoURL, "file_url": in.FileURL} {
		if val != "" && !validURL(val) {
			fields[key] = "Enter a valid http(s) URL."
		}
	}
	if in.Status != "" && in.Status != "draft" && in.Status != "submitted" {
		fields["status"] = "Status must be draft or submitted."
	}
	if len(fields) > 0 {
		return apperr.InvalidFields(fields)
	}
	return nil
}

func submissionOpen(ev db.GetEventByIDRow) error {
	if ev.Status != "published" {
		return apperr.Invalid("submission", "Submissions are closed for this event.")
	}
	if ev.SubmissionDeadline != nil && ev.SubmissionDeadline.Before(time.Now()) {
		return apperr.Invalid("submission", "The submission deadline has passed.")
	}
	return nil
}

func normalizedStatus(s string) string {
	if s == "draft" {
		return "draft"
	}
	return "submitted"
}

func validURL(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

func view(s db.Submission) View {
	return View{
		ID: s.ID, EventID: s.EventID, RegistrationID: s.RegistrationID, TeamID: s.TeamID,
		CreatedBy: s.CreatedBy, Title: s.Title, Description: s.Description,
		RepoURL: s.RepoUrl, DemoURL: s.DemoUrl, FileURL: s.FileUrl, Status: s.Status,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
