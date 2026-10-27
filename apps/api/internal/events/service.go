package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
)

type Criterion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type JudgingConfig struct {
	TargetComparisons int `json:"target_comparisons"`
	JudgeBudget       int `json:"judge_budget"`
	AnchorCount       int `json:"anchor_count"`
}

var DefaultJudgingConfig = JudgingConfig{TargetComparisons: 6, JudgeBudget: 20, AnchorCount: 3}

type Service struct {
	store *db.Store
	cfg   *config.Config
	log   *slog.Logger
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{store: store, cfg: cfg, log: log}
}

type EventFields struct {
	Title                string      `json:"title"`
	Description          string      `json:"description"`
	Category             string      `json:"category"`
	ParticipationFormat  string      `json:"participation_format"`
	Eligibility          string      `json:"eligibility"`
	Rules                string      `json:"rules"`
	ContactEmail         string      `json:"contact_email"`
	Location             string      `json:"location"`
	MinTeamSize          int16       `json:"min_team_size"`
	MaxTeamSize          int16       `json:"max_team_size"`
	RegistrationOpensAt  *time.Time  `json:"registration_opens_at"`
	RegistrationClosesAt *time.Time  `json:"registration_closes_at"`
	StartsAt             *time.Time  `json:"starts_at"`
	EndsAt               *time.Time  `json:"ends_at"`
	SubmissionDeadline   *time.Time  `json:"submission_deadline"`
	Criteria             []Criterion `json:"criteria"`
}

type EventView struct {
	ID                   uuid.UUID     `json:"id"`
	Slug                 string        `json:"slug"`
	OrganizerID          uuid.UUID     `json:"organizer_id"`
	OrganizerUsername    string        `json:"organizer_username"`
	Title                string        `json:"title"`
	Description          string        `json:"description"`
	Category             string        `json:"category"`
	ParticipationFormat  string        `json:"participation_format"`
	Status               string        `json:"status"`
	Eligibility          string        `json:"eligibility"`
	Rules                string        `json:"rules"`
	ContactEmail         string        `json:"contact_email"`
	Location             string        `json:"location"`
	MinTeamSize          int16         `json:"min_team_size"`
	MaxTeamSize          int16         `json:"max_team_size"`
	RegistrationOpensAt  *time.Time    `json:"registration_opens_at"`
	RegistrationClosesAt *time.Time    `json:"registration_closes_at"`
	StartsAt             *time.Time    `json:"starts_at"`
	EndsAt               *time.Time    `json:"ends_at"`
	SubmissionDeadline   *time.Time    `json:"submission_deadline"`
	Criteria             []Criterion   `json:"criteria"`
	JudgingStatus        string        `json:"judging_status"`
	JudgingConfig        JudgingConfig `json:"judging_config"`
	ResultsPublishedAt   *time.Time    `json:"results_published_at"`
	RegistrationStatus   string        `json:"registration_status"`
	RegistrationCount    int32         `json:"registration_count"`
	SubmissionCount      int32         `json:"submission_count"`
	CanManage            bool          `json:"can_manage"`
	IsJudge              bool          `json:"is_judge"`
	IsRegistered         bool          `json:"is_registered"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	FormFields           []FormField   `json:"form_fields,omitempty"`
}

type FormField struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	FieldType string    `json:"field_type"`
	Required  bool      `json:"required"`
	Options   []string  `json:"options"`
	HelpText  string    `json:"help_text"`
	Position  int32     `json:"position"`
}

type FormFieldInput struct {
	Label     string   `json:"label"`
	FieldType string   `json:"field_type"`
	Required  bool     `json:"required"`
	Options   []string `json:"options"`
	HelpText  string   `json:"help_text"`
}

type StaffView struct {
	ID        uuid.UUID  `json:"id"`
	EventID   uuid.UUID  `json:"event_id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	UserID    *uuid.UUID `json:"user_id"`
	Username  *string    `json:"username,omitempty"`
	AcceptURL string     `json:"accept_url,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type EventFilters struct {
	Search    string
	Category  string
	Format    string
	RegStatus string
	Limit     int32
	Offset    int32
}

// ── Validation ─────────────────────────────────────────────────────────────

func validateEventFields(in *EventFields) error {
	fields := map[string]string{}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Eligibility = strings.TrimSpace(in.Eligibility)
	in.Rules = strings.TrimSpace(in.Rules)
	in.ContactEmail = strings.ToLower(strings.TrimSpace(in.ContactEmail))
	in.Location = strings.TrimSpace(in.Location)

	if len(in.Title) < 3 || len(in.Title) > 160 {
		fields["title"] = "Title must be 3–160 characters."
	}
	if len(in.Description) > 10_000 {
		fields["description"] = "Keep the description under 10,000 characters."
	}
	if in.Category == "" || len(in.Category) > 60 {
		fields["category"] = "Pick a category."
	}
	switch in.ParticipationFormat {
	case "individual", "team", "both":
	default:
		fields["participation_format"] = "Choose individual, team, or both."
	}
	if in.ContactEmail != "" {
		if _, err := mail.ParseAddress(in.ContactEmail); err != nil {
			fields["contact_email"] = "Enter a valid email address."
		}
	}
	if in.MinTeamSize < 1 || in.MinTeamSize > 64 {
		fields["min_team_size"] = "Minimum team size must be 1–64."
	}
	if in.MaxTeamSize < 1 || in.MaxTeamSize > 64 {
		fields["max_team_size"] = "Maximum team size must be 1–64."
	}
	if in.MaxTeamSize < in.MinTeamSize {
		fields["max_team_size"] = "Maximum must be at least the minimum."
	}
	if in.ParticipationFormat == "individual" {
		if in.MinTeamSize != 1 || in.MaxTeamSize != 1 {
			fields["max_team_size"] = "Individual events use a team size of exactly 1."
		}
	}
	if in.ParticipationFormat == "team" && in.MinTeamSize < 2 {
		fields["min_team_size"] = "Team events need at least 2 people."
	}
	if in.RegistrationOpensAt != nil && in.RegistrationClosesAt != nil && !in.RegistrationClosesAt.After(*in.RegistrationOpensAt) {
		fields["registration_closes_at"] = "Registration must close after it opens."
	}
	if in.StartsAt != nil && in.EndsAt != nil && in.EndsAt.Before(*in.StartsAt) {
		fields["ends_at"] = "The end must be after the start."
	}
	if in.RegistrationClosesAt != nil && in.SubmissionDeadline != nil && in.SubmissionDeadline.Before(*in.RegistrationClosesAt) {
		fields["submission_deadline"] = "Submissions cannot close before registration does."
	}
	if len(in.Criteria) > 8 {
		fields["criteria"] = "Use at most 8 criteria."
	}
	clean := make([]Criterion, 0, len(in.Criteria))
	for _, c := range in.Criteria {
		c.Name = strings.TrimSpace(c.Name)
		c.Description = strings.TrimSpace(c.Description)
		if c.Name == "" || len(c.Name) > 60 {
			fields["criteria"] = "Each criterion needs a name under 60 characters."
			continue
		}
		if len(c.Description) > 300 {
			fields["criteria"] = "Criterion descriptions are limited to 300 characters."
			continue
		}
		clean = append(clean, c)
	}
	in.Criteria = clean
	if len(fields) > 0 {
		return apperr.InvalidFields(fields)
	}
	return nil
}

func validateFormFields(in []FormFieldInput) error {
	if len(in) > 30 {
		return apperr.Invalid("form_fields", "Use at most 30 fields.")
	}
	fields := map[string]string{}
	for i, f := range in {
		key := fmt.Sprintf("form_fields[%d]", i)
		f.Label = strings.TrimSpace(f.Label)
		if f.Label == "" || len(f.Label) > 120 {
			fields[key] = "Each field needs a label under 120 characters."
		}
		switch f.FieldType {
		case "text", "long_text", "select", "multi_select", "url", "number", "file":
		default:
			fields[key] = "Unsupported field type."
		}
		if (f.FieldType == "select" || f.FieldType == "multi_select") && len(f.Options) < 2 {
			fields[key] = "Select fields need at least two options."
		}
		if len(f.Options) > 50 {
			fields[key] = "Use at most 50 options."
		}
	}
	if len(fields) > 0 {
		return apperr.InvalidFields(fields)
	}
	return nil
}

// ── Create / update ────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, organizerID uuid.UUID, in EventFields) (*EventView, error) {
	if err := validateEventFields(&in); err != nil {
		return nil, err
	}
	criteria, err := json.Marshal(in.Criteria)
	if err != nil {
		return nil, err
	}
	var row db.CreateEventRow
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		slug := slugify(in.Title) + "-" + randomHex(3)
		row, lastErr = s.store.CreateEvent(ctx, db.CreateEventParams{
			Slug: slug, OrganizerID: organizerID, Title: in.Title,
			Description: in.Description, Category: in.Category,
			ParticipationFormat: in.ParticipationFormat, Eligibility: in.Eligibility,
			Rules: in.Rules, ContactEmail: in.ContactEmail, Location: in.Location,
			MinTeamSize: in.MinTeamSize, MaxTeamSize: in.MaxTeamSize,
			RegistrationOpensAt: in.RegistrationOpensAt, RegistrationClosesAt: in.RegistrationClosesAt,
			StartsAt: in.StartsAt, EndsAt: in.EndsAt, SubmissionDeadline: in.SubmissionDeadline,
			Criteria: criteria,
		})
		if lastErr == nil {
			break
		}
		if !isUnique(lastErr) {
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	_ = s.store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ActorID: &organizerID, EventID: &row.ID, Action: "event.created",
		Metadata: []byte(`{}`),
	})
	v := viewFromCreate(row)
	v.CanManage = true
	return &v, nil
}

func (s *Service) Update(ctx context.Context, userID, eventID uuid.UUID, in EventFields) (*EventView, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return nil, err
	}
	if ev.Status == "results_published" {
		return nil, apperr.ErrConflict
	}
	if err := validateEventFields(&in); err != nil {
		return nil, err
	}
	// If registrations exist, team sizes may not shrink below current rosters.
	if in.MaxTeamSize < ev.MaxTeamSize {
		var biggest int32
		row := s.store.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(c),0)::int FROM (
			SELECT count(*)::int AS c FROM team_members m JOIN teams t ON t.id=m.team_id
			WHERE t.event_id=$1 GROUP BY m.team_id) x`, eventID)
		if err := row.Scan(&biggest); err != nil {
			return nil, err
		}
		if biggest > int32(in.MaxTeamSize) {
			return nil, apperr.Invalid("max_team_size", fmt.Sprintf("A team already has %d members.", biggest))
		}
	}
	criteria, err := json.Marshal(in.Criteria)
	if err != nil {
		return nil, err
	}
	row, err := s.store.UpdateEvent(ctx, db.UpdateEventParams{
		ID: eventID, Title: in.Title, Description: in.Description, Category: in.Category,
		ParticipationFormat: in.ParticipationFormat, Eligibility: in.Eligibility,
		Rules: in.Rules, ContactEmail: in.ContactEmail, Location: in.Location,
		MinTeamSize: in.MinTeamSize, MaxTeamSize: in.MaxTeamSize,
		RegistrationOpensAt: in.RegistrationOpensAt, RegistrationClosesAt: in.RegistrationClosesAt,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, SubmissionDeadline: in.SubmissionDeadline,
		Criteria: criteria,
	})
	if err != nil {
		return nil, err
	}
	_ = s.store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ActorID: &userID, EventID: &eventID, Action: "event.updated", Metadata: []byte(`{}`),
	})
	v := viewFromUpdate(row)
	return &v, nil
}

func (s *Service) SetStatus(ctx context.Context, userID, eventID uuid.UUID, target string) error {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return err
	}
	switch target {
	case "published":
		if ev.Status != "draft" {
			return apperr.ErrConflict
		}
		fields := map[string]string{}
		if ev.StartsAt == nil {
			fields["starts_at"] = "Set a start date before publishing."
		}
		if ev.RegistrationClosesAt == nil {
			fields["registration_closes_at"] = "Set a registration deadline before publishing."
		}
		if ev.RegistrationClosesAt != nil && ev.RegistrationClosesAt.Before(time.Now()) {
			fields["registration_closes_at"] = "The registration deadline is already in the past."
		}
		if ev.SubmissionDeadline == nil {
			fields["submission_deadline"] = "Set a submission deadline before publishing."
		}
		if len(fields) > 0 {
			return apperr.InvalidFields(fields)
		}
	case "draft":
		if ev.Status != "published" {
			return apperr.ErrConflict
		}
		count, err := s.store.CountEventRegistrations(ctx, eventID)
		if err != nil {
			return err
		}
		if count > 0 {
			return apperr.Invalid("status", "You cannot unpublish after participants register.")
		}
	case "closed":
		if ev.Status != "published" {
			return apperr.ErrConflict
		}
	default:
		return apperr.Invalid("status", "Unsupported status transition.")
	}
	if err := s.store.UpdateEventStatus(ctx, db.UpdateEventStatusParams{ID: eventID, Status: target}); err != nil {
		return err
	}
	_ = s.store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ActorID: &userID, EventID: &eventID, Action: "event.status." + target, Metadata: []byte(`{}`),
	})
	return nil
}

func (s *Service) Delete(ctx context.Context, userID, eventID uuid.UUID) error {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return err
	}
	if ev.Status != "draft" {
		return apperr.Invalid("status", "Only draft events can be deleted. Close the event instead.")
	}
	return s.store.DeleteEvent(ctx, eventID)
}

// ── Reads ──────────────────────────────────────────────────────────────────

func (s *Service) List(ctx context.Context, f EventFilters) ([]EventView, int64, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 24
	}
	var search *string
	if v := strings.TrimSpace(f.Search); v != "" {
		if len(v) > 120 {
			return nil, 0, apperr.Invalid("q", "Search is limited to 120 characters.")
		}
		search = &v
	}
	var category, format, regStatus *string
	if f.Category != "" {
		category = &f.Category
	}
	if f.Format != "" {
		switch f.Format {
		case "individual", "team", "both":
			format = &f.Format
		default:
			return nil, 0, apperr.Invalid("format", "Unknown format filter.")
		}
	}
	if f.RegStatus != "" {
		switch f.RegStatus {
		case "upcoming", "open", "closed", "ended":
			regStatus = &f.RegStatus
		default:
			return nil, 0, apperr.Invalid("status", "Unknown registration status filter.")
		}
	}
	rows, err := s.store.ListPublishedEvents(ctx, db.ListPublishedEventsParams{
		Search: search, Category: category, Format: format, RegStatus: regStatus,
		Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]EventView, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.TotalCount
		v := eventView(r.ID, r.Slug, r.OrganizerID, r.Title, r.Description, r.Category,
			r.ParticipationFormat, r.Status, r.Eligibility, r.Rules, r.ContactEmail,
			r.Location, r.MinTeamSize, r.MaxTeamSize, r.RegistrationOpensAt,
			r.RegistrationClosesAt, r.StartsAt, r.EndsAt, r.SubmissionDeadline,
			r.Criteria, r.JudgingConfig, r.JudgingStatus, r.ResultsPublishedAt,
			r.CreatedAt, r.UpdatedAt)
		out = append(out, v)
	}
	return out, total, nil
}

// Get returns a public event. Drafts are only visible to organizers/judges.
func (s *Service) Get(ctx context.Context, viewerID *uuid.UUID, eventID uuid.UUID) (*EventView, error) {
	row, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return s.hydrate(ctx, viewerID, row, true)
}

// GetManage returns a draft or published event with organizer-only detail.
func (s *Service) GetManage(ctx context.Context, userID, eventID uuid.UUID) (*EventView, error) {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return nil, err
	}
	row, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	v, err := s.hydrate(ctx, &userID, row, false)
	if err != nil {
		return nil, err
	}
	v.CanManage = true
	return v, nil
}

func (s *Service) hydrate(ctx context.Context, viewerID *uuid.UUID, row db.GetEventByIDRow, includeFields bool) (*EventView, error) {
	if row.Status == "draft" {
		if viewerID == nil {
			return nil, apperr.ErrNotFound
		}
		allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: *viewerID, ID: row.ID})
		if err != nil {
			return nil, err
		}
		if !allowed {
			judge, _ := s.store.IsJudgeForEvent(ctx, db.IsJudgeForEventParams{EventID: row.ID, UserID: viewerID})
			if !judge {
				return nil, apperr.ErrNotFound
			}
		}
	}
	v := eventView(row.ID, row.Slug, row.OrganizerID, row.Title, row.Description, row.Category,
		row.ParticipationFormat, row.Status, row.Eligibility, row.Rules, row.ContactEmail,
		row.Location, row.MinTeamSize, row.MaxTeamSize, row.RegistrationOpensAt,
		row.RegistrationClosesAt, row.StartsAt, row.EndsAt, row.SubmissionDeadline,
		row.Criteria, row.JudgingConfig, row.JudgingStatus, row.ResultsPublishedAt,
		row.CreatedAt, row.UpdatedAt)

	if u, err := s.store.GetUserByID(ctx, row.OrganizerID); err == nil {
		v.OrganizerUsername = u.Username
	}
	regCount, _ := s.store.CountEventRegistrations(ctx, row.ID)
	subCount, _ := s.store.CountEventSubmissions(ctx, row.ID)
	v.RegistrationCount = regCount
	v.SubmissionCount = subCount
	if includeFields {
		fields, err := s.store.ListFormFields(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		v.FormFields = formFieldViews(fields)
	}
	if viewerID != nil {
		canManage, _ := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: *viewerID, ID: row.ID})
		isJudge, _ := s.store.IsJudgeForEvent(ctx, db.IsJudgeForEventParams{EventID: row.ID, UserID: viewerID})
		v.CanManage = canManage
		v.IsJudge = isJudge
		if reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: row.ID, UserID: *viewerID}); err == nil && reg.Status != "cancelled" {
			v.IsRegistered = true
		}
	}
	return &v, nil
}

func (s *Service) ListHosted(ctx context.Context, userID uuid.UUID) ([]EventView, error) {
	rows, err := s.store.ListEventsByOrganizer(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]EventView, 0, len(rows))
	for _, r := range rows {
		v := eventView(r.ID, r.Slug, r.OrganizerID, r.Title, r.Description, r.Category,
			r.ParticipationFormat, r.Status, r.Eligibility, r.Rules, r.ContactEmail,
			r.Location, r.MinTeamSize, r.MaxTeamSize, r.RegistrationOpensAt,
			r.RegistrationClosesAt, r.StartsAt, r.EndsAt, r.SubmissionDeadline,
			r.Criteria, r.JudgingConfig, r.JudgingStatus, r.ResultsPublishedAt,
			r.CreatedAt, r.UpdatedAt)
		v.CanManage = true
		regCount, _ := s.store.CountEventRegistrations(ctx, r.ID)
		subCount, _ := s.store.CountEventSubmissions(ctx, r.ID)
		v.RegistrationCount = regCount
		v.SubmissionCount = subCount
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) ListForJudge(ctx context.Context, userID uuid.UUID) ([]EventView, error) {
	rows, err := s.store.ListJudgeEvents(ctx, &userID)
	if err != nil {
		return nil, err
	}
	out := make([]EventView, 0, len(rows))
	for _, r := range rows {
		v := eventView(r.ID, r.Slug, r.OrganizerID, r.Title, r.Description, r.Category,
			r.ParticipationFormat, r.Status, r.Eligibility, r.Rules, r.ContactEmail,
			r.Location, r.MinTeamSize, r.MaxTeamSize, r.RegistrationOpensAt,
			r.RegistrationClosesAt, r.StartsAt, r.EndsAt, r.SubmissionDeadline,
			r.Criteria, r.JudgingConfig, r.JudgingStatus, r.ResultsPublishedAt,
			r.CreatedAt, r.UpdatedAt)
		v.IsJudge = true
		out = append(out, v)
	}
	return out, nil
}

// ── Form builder ───────────────────────────────────────────────────────────

func (s *Service) ReplaceForm(ctx context.Context, userID, eventID uuid.UUID, inputs []FormFieldInput) ([]FormField, error) {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return nil, err
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if ev.Status == "results_published" {
		return nil, apperr.ErrConflict
	}
	if err := validateFormFields(inputs); err != nil {
		return nil, err
	}
	created := make([]db.EventFormField, 0, len(inputs))
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteFormFieldsForEvent(ctx, eventID); err != nil {
			return err
		}
		for i, in := range inputs {
			options, err := json.Marshal(in.Options)
			if err != nil {
				return err
			}
			f, err := q.CreateFormField(ctx, db.CreateFormFieldParams{
				EventID: eventID, Label: strings.TrimSpace(in.Label), FieldType: in.FieldType,
				Required: in.Required, Options: options, HelpText: strings.TrimSpace(in.HelpText),
				Position: int32(i),
			})
			if err != nil {
				return err
			}
			created = append(created, f)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return formFieldViews(created), nil
}

func (s *Service) FormFields(ctx context.Context, eventID uuid.UUID) ([]FormField, error) {
	fields, err := s.store.ListFormFields(ctx, eventID)
	if err != nil {
		return nil, err
	}
	return formFieldViews(fields), nil
}

// ── Staff (judges / co-organizers) ─────────────────────────────────────────

func (s *Service) InviteStaff(ctx context.Context, userID, eventID uuid.UUID, email, role string) (*StaffView, error) {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, apperr.Invalid("email", "Enter a valid email address.")
	}
	if role != "judge" && role != "co_organizer" {
		return nil, apperr.Invalid("role", "Role must be judge or co_organizer.")
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if ev.Status == "results_published" {
		return nil, apperr.ErrConflict
	}
	var invitedUserID *uuid.UUID
	if u, err := s.store.GetUserByEmail(ctx, email); err == nil {
		if u.ID == userID {
			return nil, apperr.Invalid("email", "You are already on this event.")
		}
		invitedUserID = &u.ID
	}
	token := randomHex(24)
	staff, err := s.store.CreateStaffInvite(ctx, db.CreateStaffInviteParams{
		EventID: eventID, UserID: invitedUserID, Email: email, Role: role,
		InviteToken: &token, InvitedBy: userID,
	})
	if err != nil {
		return nil, err
	}
	if invitedUserID != nil {
		_, _ = s.store.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: *invitedUserID, Kind: "staff_invite",
			Title: "You're invited to " + ev.Title,
			Body:  "Accept the " + role + " invite to join the event.",
			Link:  "/invites/staff/" + token,
		})
	}
	_ = s.store.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ActorID: &userID, EventID: &eventID, Action: "staff.invited", Metadata: []byte(`{}`),
	})
	return &StaffView{
		ID: staff.ID, EventID: staff.EventID, Email: staff.Email, Role: staff.Role,
		Status: staff.Status, UserID: staff.UserID, AcceptURL: "/invites/staff/" + token,
		CreatedAt: staff.CreatedAt,
	}, nil
}

func (s *Service) ListStaff(ctx context.Context, userID, eventID uuid.UUID, role string) ([]StaffView, error) {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return nil, err
	}
	staff, err := s.store.ListEventStaff(ctx, db.ListEventStaffParams{EventID: eventID, Role: role})
	if err != nil {
		return nil, err
	}
	out := make([]StaffView, 0, len(staff))
	for _, st := range staff {
		v := StaffView{
			ID: st.ID, EventID: st.EventID, Email: st.Email, Role: st.Role,
			Status: st.Status, UserID: st.UserID, CreatedAt: st.CreatedAt,
		}
		if st.UserID != nil {
			if u, err := s.store.GetUserByID(ctx, *st.UserID); err == nil {
				v.Username = &u.Username
			}
		}
		if st.Status == "invited" && st.InviteToken != nil {
			v.AcceptURL = "/invites/staff/" + *st.InviteToken
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) AcceptStaffInvite(ctx context.Context, userID uuid.UUID, token string) (*uuid.UUID, string, error) {
	staff, err := s.store.GetStaffByToken(ctx, &token)
	if err != nil {
		return nil, "", apperr.ErrNotFound
	}
	if staff.Status != "invited" {
		return nil, "", apperr.ErrConflict
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if !strings.EqualFold(user.Email, staff.Email) {
		return nil, "", apperr.ErrForbidden
	}
	if err := s.store.AcceptStaffInvite(ctx, db.AcceptStaffInviteParams{ID: staff.ID, UserID: &userID}); err != nil {
		return nil, "", err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: userID, Kind: "staff_accepted", Title: "Invite accepted",
		Body: "You joined the event team.", Link: "/events/" + staff.EventID.String(),
	}); err != nil {
		s.log.Warn("notify staff accept", "error", err)
	}
	return &staff.EventID, staff.Role, nil
}

func (s *Service) RemoveStaff(ctx context.Context, userID, eventID, staffID uuid.UUID) error {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return err
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.JudgingStatus == "locked" || ev.JudgingStatus == "published" {
		return apperr.ErrConflict
	}
	staff, err := s.store.ListEventStaff(ctx, db.ListEventStaffParams{EventID: eventID, Role: "judge"})
	if err == nil {
		for _, st := range staff {
			if st.ID == staffID {
				return s.store.RemoveStaff(ctx, staffID)
			}
		}
	}
	staff, err = s.store.ListEventStaff(ctx, db.ListEventStaffParams{EventID: eventID, Role: "co_organizer"})
	if err == nil {
		for _, st := range staff {
			if st.ID == staffID {
				return s.store.RemoveStaff(ctx, staffID)
			}
		}
	}
	return apperr.ErrNotFound
}

// ── Judging config ─────────────────────────────────────────────────────────

func (s *Service) GetJudgingConfig(ctx context.Context, userID uuid.UUID, eventID uuid.UUID) (JudgingConfig, []Criterion, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return JudgingConfig{}, nil, apperr.ErrNotFound
	}
	canManage, _ := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: eventID})
	isJudge, _ := s.store.IsJudgeForEvent(ctx, db.IsJudgeForEventParams{EventID: eventID, UserID: &userID})
	if !canManage && !isJudge {
		return JudgingConfig{}, nil, apperr.ErrNotFound
	}
	return parseJudgingConfig(ev.JudgingConfig), parseCriteria(ev.Criteria), nil
}

func (s *Service) SetJudgingConfig(ctx context.Context, userID, eventID uuid.UUID, cfgIn JudgingConfig) (JudgingConfig, error) {
	if err := s.requireManage(ctx, userID, eventID); err != nil {
		return JudgingConfig{}, err
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return JudgingConfig{}, apperr.ErrNotFound
	}
	if ev.JudgingStatus == "locked" || ev.JudgingStatus == "published" {
		return JudgingConfig{}, apperr.ErrConflict
	}
	fields := map[string]string{}
	if cfgIn.TargetComparisons < 2 || cfgIn.TargetComparisons > 50 {
		fields["target_comparisons"] = "Target comparisons per submission must be 2–50."
	}
	if cfgIn.JudgeBudget < 2 || cfgIn.JudgeBudget > 500 {
		fields["judge_budget"] = "Comparisons per judge must be 2–500."
	}
	if cfgIn.AnchorCount < 1 || cfgIn.AnchorCount > 50 {
		fields["anchor_count"] = "Anchor submissions must be 1–50."
	}
	if len(fields) > 0 {
		return JudgingConfig{}, apperr.InvalidFields(fields)
	}
	raw, err := json.Marshal(cfgIn)
	if err != nil {
		return JudgingConfig{}, err
	}
	if err := s.store.UpdateEventJudgingConfig(ctx, db.UpdateEventJudgingConfigParams{ID: eventID, JudgingConfig: raw}); err != nil {
		return JudgingConfig{}, err
	}
	return cfgIn, nil
}

func (s *Service) CanManage(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	return s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: eventID})
}

func (s *Service) requireManage(ctx context.Context, userID, eventID uuid.UUID) error {
	allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: eventID})
	if err != nil {
		return err
	}
	if !allowed {
		return apperr.ErrNotFound
	}
	return nil
}

// ── Mapping helpers ────────────────────────────────────────────────────────

func eventView(id uuid.UUID, slug string, organizerID uuid.UUID, title, description, category, format, status, eligibility, rules, contactEmail, location string, minTeam, maxTeam int16, regOpens, regCloses, startsAt, endsAt, submissionDeadline *time.Time, criteriaRaw, judgingRaw []byte, judgingStatus string, resultsPublishedAt *time.Time, createdAt, updatedAt time.Time) EventView {
	return EventView{
		ID: id, Slug: slug, OrganizerID: organizerID, Title: title, Description: description,
		Category: category, ParticipationFormat: format, Status: status,
		Eligibility: eligibility, Rules: rules, ContactEmail: contactEmail, Location: location,
		MinTeamSize: minTeam, MaxTeamSize: maxTeam,
		RegistrationOpensAt: regOpens, RegistrationClosesAt: regCloses,
		StartsAt: startsAt, EndsAt: endsAt, SubmissionDeadline: submissionDeadline,
		Criteria: parseCriteria(criteriaRaw), JudgingStatus: judgingStatus,
		JudgingConfig: parseJudgingConfig(judgingRaw), ResultsPublishedAt: resultsPublishedAt,
		RegistrationStatus: registrationStatus(status, regOpens, regCloses, endsAt),
		CreatedAt:          createdAt, UpdatedAt: updatedAt,
	}
}

func viewFromCreate(r db.CreateEventRow) EventView {
	return eventView(r.ID, r.Slug, r.OrganizerID, r.Title, r.Description, r.Category,
		r.ParticipationFormat, r.Status, r.Eligibility, r.Rules, r.ContactEmail, r.Location,
		r.MinTeamSize, r.MaxTeamSize, r.RegistrationOpensAt, r.RegistrationClosesAt,
		r.StartsAt, r.EndsAt, r.SubmissionDeadline, r.Criteria, r.JudgingConfig,
		r.JudgingStatus, r.ResultsPublishedAt, r.CreatedAt, r.UpdatedAt)
}

func viewFromUpdate(r db.UpdateEventRow) EventView {
	return eventView(r.ID, r.Slug, r.OrganizerID, r.Title, r.Description, r.Category,
		r.ParticipationFormat, r.Status, r.Eligibility, r.Rules, r.ContactEmail, r.Location,
		r.MinTeamSize, r.MaxTeamSize, r.RegistrationOpensAt, r.RegistrationClosesAt,
		r.StartsAt, r.EndsAt, r.SubmissionDeadline, r.Criteria, r.JudgingConfig,
		r.JudgingStatus, r.ResultsPublishedAt, r.CreatedAt, r.UpdatedAt)
}

func formFieldViews(fields []db.EventFormField) []FormField {
	out := make([]FormField, 0, len(fields))
	for _, f := range fields {
		var options []string
		_ = json.Unmarshal(f.Options, &options)
		if options == nil {
			options = []string{}
		}
		out = append(out, FormField{
			ID: f.ID, Label: f.Label, FieldType: f.FieldType, Required: f.Required,
			Options: options, HelpText: f.HelpText, Position: f.Position,
		})
	}
	return out
}

func parseCriteria(raw []byte) []Criterion {
	var c []Criterion
	if len(raw) == 0 || json.Unmarshal(raw, &c) != nil || c == nil {
		return []Criterion{}
	}
	return c
}

func parseJudgingConfig(raw []byte) JudgingConfig {
	c := DefaultJudgingConfig
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	if c.TargetComparisons <= 0 {
		c.TargetComparisons = DefaultJudgingConfig.TargetComparisons
	}
	if c.JudgeBudget <= 0 {
		c.JudgeBudget = DefaultJudgingConfig.JudgeBudget
	}
	if c.AnchorCount <= 0 {
		c.AnchorCount = DefaultJudgingConfig.AnchorCount
	}
	return c
}

func registrationStatus(eventStatus string, opens, closes, ends *time.Time) string {
	now := time.Now()
	switch {
	case eventStatus == "results_published" || (ends != nil && ends.Before(now)):
		return "ended"
	case eventStatus == "closed" || (closes != nil && closes.Before(now)):
		return "closed"
	case opens != nil && opens.After(now):
		return "upcoming"
	default:
		return "open"
	}
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	slug := strings.ToLower(slugStrip.ReplaceAllString(title, "-"))
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug == "" {
		slug = "event"
	}
	return slug
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// SortedCategories is exposed to the frontend so filters stay consistent.
func SortedCategories() []string {
	c := []string{"Hackathon", "Competition", "Contest", "Workshop", "Other"}
	sort.Strings(c)
	return c
}
