package registrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
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

type View struct {
	ID            uuid.UUID       `json:"id"`
	EventID       uuid.UUID       `json:"event_id"`
	EventTitle    string          `json:"event_title,omitempty"`
	EventSlug     string          `json:"event_slug,omitempty"`
	EventStatus   string          `json:"event_status,omitempty"`
	StartsAt      *time.Time      `json:"starts_at,omitempty"`
	UserID        uuid.UUID       `json:"user_id"`
	Username      string          `json:"username,omitempty"`
	FullName      string          `json:"full_name,omitempty"`
	College       string          `json:"college,omitempty"`
	Email         string          `json:"email,omitempty"`
	TeamID        *uuid.UUID      `json:"team_id"`
	TeamName      string          `json:"team_name,omitempty"`
	Answers       json.RawMessage `json:"answers"`
	Status        string          `json:"status"`
	HasSubmission bool            `json:"has_submission"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (s *Service) Create(ctx context.Context, userID, eventID uuid.UUID, teamID *uuid.UUID, answers map[string]any) (*View, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := registrationOpen(ev); err != nil {
		return nil, err
	}
	if ev.ParticipationFormat == "individual" && teamID != nil {
		return nil, apperr.Invalid("team_id", "This event is individual-only.")
	}
	existing, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err == nil && existing.Status != "cancelled" {
		return nil, apperr.Invalid("registration", "You are already registered for this event.")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	fields, err := s.store.ListFormFields(ctx, db.ListFormFieldsParams{EventID: eventID, FormKind: "registration"})
	if err != nil {
		return nil, err
	}
	if ev.MergeRegistrationSubmission {
		extra, err := s.store.ListFormFields(ctx, db.ListFormFieldsParams{EventID: eventID, FormKind: "submission"})
		if err != nil {
			return nil, err
		}
		fields = append(fields, extra...)
	}
	cleanAnswers, err := validateAnswers(fields, answers)
	if err != nil {
		return nil, err
	}
	if err := s.autofill(ctx, userID, cleanAnswers); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cleanAnswers)
	if err != nil {
		return nil, err
	}

	var members []uuid.UUID
	if teamID != nil {
		team, err := s.store.GetTeamByID(ctx, *teamID)
		if err != nil {
			return nil, apperr.Invalid("team_id", "Team not found.")
		}
		if team.EventID != eventID {
			return nil, apperr.Invalid("team_id", "That team belongs to another event.")
		}
		if _, err := s.store.GetTeamMember(ctx, db.GetTeamMemberParams{TeamID: *teamID, UserID: userID}); err != nil {
			return nil, apperr.ErrNotFound
		}
		count, err := s.store.CountTeamMembers(ctx, *teamID)
		if err != nil {
			return nil, err
		}
		if count < int32(ev.MinTeamSize) {
			return nil, apperr.Invalid("team_id", fmt.Sprintf("This team needs at least %d members to register.", ev.MinTeamSize))
		}
		if _, err := s.store.GetTeamRegistration(ctx, db.GetTeamRegistrationParams{EventID: eventID, TeamID: teamID}); err == nil {
			return nil, apperr.Invalid("team_id", "This team is already registered.")
		}
		ids, err := s.store.ListTeamMemberIDs(ctx, *teamID)
		if err != nil {
			return nil, err
		}
		members = ids
		for _, memberID := range members {
			if reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: memberID}); err == nil && reg.Status != "cancelled" {
				return nil, apperr.Invalid("team_id", "A teammate is already registered individually. Ask them to cancel first.")
			}
		}
	} else {
		members = []uuid.UUID{userID}
	}

	var created db.Registration
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		// Serialize with event status changes. The pre-transaction read above
		// is only for fast validation; this one is authoritative.
		locked, err := q.LockEventForUpdate(ctx, eventID)
		if err != nil {
			return apperr.ErrNotFound
		}
		if err := registrationOpenAt(locked.Status, locked.RegistrationOpensAt, locked.RegistrationClosesAt); err != nil {
			return err
		}
		if teamID != nil {
			if _, err := q.LockTeamForUpdate(ctx, *teamID); err != nil {
				return apperr.ErrNotFound
			}
			count, err := q.CountTeamMembers(ctx, *teamID)
			if err != nil {
				return err
			}
			if count < int32(locked.MinTeamSize) {
				return apperr.Invalid("team_id", "The team no longer meets the minimum size.")
			}
		}
		for _, memberID := range members {
			reg, err := q.CreateRegistrationForTeamMember(ctx, db.CreateRegistrationForTeamMemberParams{
				EventID: eventID, UserID: memberID, TeamID: teamID, FormAnswers: raw,
			})
			if err != nil {
				if isUnique(err) {
					return apperr.Invalid("registration", "Someone on this team is already registered.")
				}
				return err
			}
			if memberID == userID {
				created = reg
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: userID, Kind: "registration", Title: "You're registered",
		Body: "Your registration for " + ev.Title + " is confirmed.",
		Link: "/events/" + eventID.String(),
	}); err != nil {
		s.log.Warn("registration notification", "error", err)
	}
	return s.get(ctx, created.ID)
}

func (s *Service) GetMine(ctx context.Context, userID, eventID uuid.UUID) (*View, error) {
	reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.get(ctx, reg.ID)
}

func (s *Service) UpdateAnswers(ctx context.Context, userID, eventID uuid.UUID, answers map[string]any) (*View, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := registrationOpen(ev); err != nil {
		return nil, err
	}
	reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if reg.Status == "cancelled" {
		return nil, apperr.ErrConflict
	}
	fields, err := s.store.ListFormFields(ctx, db.ListFormFieldsParams{EventID: eventID, FormKind: "registration"})
	if err != nil {
		return nil, err
	}
	if ev.MergeRegistrationSubmission {
		extra, err := s.store.ListFormFields(ctx, db.ListFormFieldsParams{EventID: eventID, FormKind: "submission"})
		if err != nil {
			return nil, err
		}
		fields = append(fields, extra...)
	}
	clean, err := validateAnswers(fields, answers)
	if err != nil {
		return nil, err
	}
	if err := s.autofill(ctx, userID, clean); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	if reg.TeamID != nil {
		if err := s.store.UpdateTeamRegistrationAnswers(ctx, db.UpdateTeamRegistrationAnswersParams{
			EventID: eventID, FormAnswers: raw, TeamID: reg.TeamID,
		}); err != nil {
			return nil, err
		}
	} else {
		if err := s.store.UpdateRegistrationAnswers(ctx, db.UpdateRegistrationAnswersParams{
			ID: reg.ID, FormAnswers: raw, UserID: userID,
		}); err != nil {
			return nil, err
		}
	}
	return s.get(ctx, reg.ID)
}

func (s *Service) Cancel(ctx context.Context, userID, eventID uuid.UUID) error {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if err := registrationOpen(ev); err != nil {
		return err
	}
	reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil {
		return apperr.ErrNotFound
	}
	if reg.TeamID != nil {
		return apperr.Invalid("registration", "Leave your team to withdraw a team registration.")
	}
	return s.store.InTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetSubmissionByRegistration(ctx, &reg.ID); err == nil {
			return apperr.Invalid("registration", "You already submitted work. Ask the organizer to cancel it.")
		}
		return q.DeleteRegistration(ctx, reg.ID)
	})
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
	rows, err := s.store.ListRegistrationsForEvent(ctx, db.ListRegistrationsForEventParams{
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
			ID: r.ID, EventID: r.EventID, UserID: r.UserID, Username: r.Username,
			Email: r.Email, FullName: deref(r.FullName), College: deref(r.College),
			TeamID: r.TeamID, TeamName: deref(r.TeamName), Answers: r.FormAnswers,
			Status: r.Status, HasSubmission: r.HasSubmission, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, total, nil
}

func (s *Service) SetStatus(ctx context.Context, userID, registrationID uuid.UUID, status string) error {
	reg, err := s.store.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, reg.EventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: reg.EventID})
	if err != nil {
		return err
	}
	if !allowed {
		return apperr.ErrNotFound
	}
	switch status {
	case "registered", "waitlisted", "cancelled", "disqualified":
	default:
		return apperr.Invalid("status", "Unknown registration status.")
	}
	if ev.Status == "results_published" {
		return apperr.ErrConflict
	}
	return s.store.UpdateRegistrationStatus(ctx, db.UpdateRegistrationStatusParams{ID: registrationID, Status: status})
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]View, error) {
	rows, err := s.store.ListMyRegistrations(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		out = append(out, View{
			ID: r.ID, EventID: r.EventID, EventTitle: r.EventTitle, EventSlug: r.EventSlug,
			EventStatus: r.EventStatus, StartsAt: r.StartsAt, UserID: r.UserID,
			TeamID: r.TeamID, TeamName: deref(r.TeamName), Answers: r.FormAnswers,
			Status: r.Status, HasSubmission: r.HasSubmission, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, registrationID uuid.UUID) (*View, error) {
	reg, err := s.store.GetRegistrationByID(ctx, registrationID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, reg.EventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	user, err := s.store.GetUserByID(ctx, reg.UserID)
	if err != nil {
		return nil, err
	}
	profile, profileErr := s.store.GetProfile(ctx, reg.UserID)
	v := View{
		ID: reg.ID, EventID: reg.EventID, EventTitle: ev.Title, EventSlug: ev.Slug,
		EventStatus: ev.Status, StartsAt: ev.StartsAt, UserID: reg.UserID,
		Email: user.Email, TeamID: reg.TeamID, Answers: reg.FormAnswers,
		Status: reg.Status, CreatedAt: reg.CreatedAt, UpdatedAt: reg.UpdatedAt,
	}
	if profileErr == nil {
		v.FullName = profile.FullName
		v.College = profile.College
	}
	if reg.TeamID != nil {
		if team, err := s.store.GetTeamByID(ctx, *reg.TeamID); err == nil {
			v.TeamName = team.Name
		}
	}
	if _, err := s.store.GetSubmissionByRegistration(ctx, &reg.ID); err == nil {
		v.HasSubmission = true
	}
	return &v, nil
}

// autofill seeds standard profile fields without overwriting user-provided values.
func (s *Service) autofill(ctx context.Context, userID uuid.UUID, answers map[string]any) error {
	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	defaults := map[string]any{
		"full_name": profile.FullName, "college": profile.College,
		"course": profile.Course, "email": user.Email,
	}
	if profile.YearOfStudy != nil {
		defaults["year_of_study"] = *profile.YearOfStudy
	}
	for k, v := range defaults {
		if _, exists := answers[k]; !exists {
			answers[k] = v
		}
	}
	return nil
}

func validateAnswers(fields []db.EventFormField, answers map[string]any) (map[string]any, error) {
	if answers == nil {
		answers = map[string]any{}
	}
	if len(answers) > 80 {
		return nil, apperr.Invalid("answers", "Too many answers.")
	}
	known := map[string]db.EventFormField{}
	for _, f := range fields {
		known[f.ID.String()] = f
	}
	// Reject unknown custom-field keys, but allow the reserved autofill keys.
	reserved := map[string]bool{"full_name": true, "college": true, "course": true, "year_of_study": true, "email": true}
	for key := range answers {
		if reserved[key] {
			continue
		}
		if _, ok := known[key]; !ok {
			return nil, apperr.Invalid("answers", "The form has changed. Reload and try again.")
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
		return nil, apperr.InvalidFields(errs)
	}
	return answers, nil
}

func registrationOpen(ev db.GetEventByIDRow) error {
	return registrationOpenAt(ev.Status, ev.RegistrationOpensAt, ev.RegistrationClosesAt)
}

func registrationOpenAt(status string, opensAt, closesAt *time.Time) error {
	now := time.Now()
	if status != "published" {
		return apperr.Invalid("registration", "Registration is not open for this event.")
	}
	if opensAt != nil && opensAt.After(now) {
		return apperr.Invalid("registration", "Registration has not opened yet.")
	}
	if closesAt != nil && closesAt.Before(now) {
		return apperr.Invalid("registration", "Registration has closed.")
	}
	return nil
}

func validURL(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
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
