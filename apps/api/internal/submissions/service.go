package submissions

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
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
	Title       string         `json:"title"`
	Description string         `json:"description"`
	RepoURL     string         `json:"repo_url"`
	DemoURL     string         `json:"demo_url"`
	FileURL     string         `json:"file_url"`
	Status      string         `json:"status"`
	Answers     map[string]any `json:"answers"`
}

type View struct {
	ID                 uuid.UUID      `json:"id"`
	EventID            uuid.UUID      `json:"event_id"`
	EventTitle         string         `json:"event_title,omitempty"`
	EventSlug          string         `json:"event_slug,omitempty"`
	SubmissionDeadline *time.Time     `json:"submission_deadline,omitempty"`
	RegistrationID     *uuid.UUID     `json:"registration_id"`
	TeamID             *uuid.UUID     `json:"team_id"`
	TeamName           string         `json:"team_name,omitempty"`
	CreatedBy          uuid.UUID      `json:"created_by"`
	CreatorUsername    string         `json:"creator_username,omitempty"`
	CreatorName        string         `json:"creator_name,omitempty"`
	Title              string         `json:"title"`
	Description        string         `json:"description"`
	RepoURL            string         `json:"repo_url"`
	DemoURL            string         `json:"demo_url"`
	FileURL            string         `json:"file_url"`
	Answers            map[string]any `json:"answers"`
	Status             string         `json:"status"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
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
	if err := s.validateAnswers(ctx, eventID, in.Answers); err != nil {
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
		Answers: marshalAnswers(in.Answers),
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
	if err := s.validateAnswers(ctx, sub.EventID, in.Answers); err != nil {
		return nil, err
	}
	updated, err := s.store.UpdateSubmission(ctx, db.UpdateSubmissionParams{
		ID: submissionID, Title: in.Title, Description: in.Description,
		RepoUrl: in.RepoURL, DemoUrl: in.DemoURL, FileUrl: in.FileURL,
		Status: normalizedStatus(in.Status), Answers: marshalAnswers(in.Answers),
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
			Answers:   unmarshalAnswers(r.Answers),
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
		switch status {
		case "draft", "submitted":
			statusPtr = &status
		default:
			return nil, 0, apperr.Invalid("status", "Status must be draft or submitted.")
		}
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
			Answers:   unmarshalAnswers(r.Answers),
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

// validateAnswers checks custom submission answers against the event's
// submission form. Keys are field IDs; unknown keys usually mean the form was
// edited in another tab, so asking the participant to reload is safest.
func (s *Service) validateAnswers(ctx context.Context, eventID uuid.UUID, answers map[string]any) error {
	fields, err := s.store.ListFormFields(ctx, db.ListFormFieldsParams{EventID: eventID, FormKind: "submission"})
	if err != nil {
		return err
	}
	if answers == nil {
		answers = map[string]any{}
	}
	if len(answers) > 80 {
		return apperr.Invalid("answers", "Too many answers.")
	}
	known := make(map[string]db.EventFormField, len(fields))
	for _, f := range fields {
		known[f.ID.String()] = f
	}
	for key := range answers {
		if _, ok := known[key]; !ok {
			return apperr.Invalid("answers", "The form has changed. Reload and try again.")
		}
	}
	errs := map[string]string{}
	for id, f := range known {
		val, present := answers[id]
		empty := !present || val == nil || val == "" || isEmptySlice(val)
		if f.Required && empty {
			errs[id] = "This field is required."
			continue
		}
		if empty {
			continue
		}
		switch f.FieldType {
		case "text", "long_text":
			str, ok := val.(string)
			if !ok {
				errs[id] = "Enter text."
				continue
			}
			if len(str) > 5000 {
				errs[id] = "Keep this under 5,000 characters."
			}
		case "url", "file":
			str, ok := val.(string)
			if !ok || !validURL(str) {
				errs[id] = "Enter a valid http(s) URL."
			}
		case "number":
			switch n := val.(type) {
			case float64:
			case string:
				if _, err := strconv.ParseFloat(n, 64); err != nil {
					errs[id] = "Enter a number."
				}
			default:
				errs[id] = "Enter a number."
			}
		case "select":
			str, ok := val.(string)
			var options []string
			_ = json.Unmarshal(f.Options, &options)
			if !ok || !contains(options, str) {
				errs[id] = "Choose one of the options."
			}
		case "multi_select":
			arr, ok := val.([]any)
			if !ok {
				errs[id] = "Choose at least one option."
				continue
			}
			var options []string
			_ = json.Unmarshal(f.Options, &options)
			for _, item := range arr {
				str, ok := item.(string)
				if !ok || !contains(options, str) {
					errs[id] = "Choose valid options."
					break
				}
			}
		}
	}
	if len(errs) > 0 {
		return apperr.InvalidFields(errs)
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func isEmptySlice(v any) bool {
	if arr, ok := v.([]any); ok && len(arr) == 0 {
		return true
	}
	if arr, ok := v.([]string); ok && len(arr) == 0 {
		return true
	}
	return false
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
		Answers:   unmarshalAnswers(s.Answers),
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func marshalAnswers(answers map[string]any) []byte {
	if answers == nil {
		answers = map[string]any{}
	}
	raw, err := json.Marshal(answers)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func unmarshalAnswers(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
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
