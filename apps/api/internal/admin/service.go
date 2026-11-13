// Package admin exposes privileged operations for local testing. It can seed a
// varied set of events, submissions, judges and results so every public filter
// and judging mode has something to exercise without hand-building accounts.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"compytition/internal/apperr"
	"compytition/internal/auth"
	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/judging"
)

const (
	// SeedPassword is shared by every generated account. These accounts only
	// exist for local testing and are deleted by the reset endpoint.
	SeedPassword = "demo1234"

	seedEventPrefix      = "seed-"
	seedEventPattern     = "seed-%"
	seedUserEmailPattern = "seed.%@compytition.local"
)

type Service struct {
	store   *db.Store
	cfg     *config.Config
	log     *slog.Logger
	judging *judging.Service
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{
		store: store, cfg: cfg, log: log,
		judging: judging.NewService(store, cfg, log),
	}
}

// Stats is the small dashboard shown above the seed controls.
type Stats struct {
	Users       int32 `json:"users"`
	Events      int32 `json:"events"`
	Submissions int32 `json:"submissions"`
	Judgments   int32 `json:"judgments"`
	Uploads     int32 `json:"uploads"`
}

// SeededEvent describes one generated event for the admin dashboard.
type SeededEvent struct {
	ID                   uuid.UUID  `json:"id"`
	Slug                 string     `json:"slug"`
	Title                string     `json:"title"`
	Category             string     `json:"category"`
	ParticipationFormat  string     `json:"participation_format"`
	Status               string     `json:"status"`
	IsRemote             bool       `json:"is_remote"`
	Country              string     `json:"country"`
	State                string     `json:"state"`
	City                 string     `json:"city"`
	RegistrationOpensAt  *time.Time `json:"registration_opens_at"`
	RegistrationClosesAt *time.Time `json:"registration_closes_at"`
	StartsAt             *time.Time `json:"starts_at"`
	EndsAt               *time.Time `json:"ends_at"`
	SubmissionDeadline   *time.Time `json:"submission_deadline"`
	JudgingStatus        string     `json:"judging_status"`
	ResultsPublishedAt   *time.Time `json:"results_published_at"`
	CreatedAt            time.Time  `json:"created_at"`
}

type Overview struct {
	Stats       Stats         `json:"stats"`
	SeedEvents  int32         `json:"seed_events"`
	SeedUsers   int32         `json:"seed_users"`
	SeedEnabled bool          `json:"seed_enabled"`
	Events      []SeededEvent `json:"events"`
}

type SeedOptions struct {
	// Events limits how many templates are generated. Zero means all of them.
	Events int
	// Reset removes existing seed events and users before creating new ones.
	Reset bool
}

type SeedResult struct {
	Events      int           `json:"events"`
	Users       int           `json:"users"`
	Submissions int           `json:"submissions"`
	Judgments   int           `json:"judgments"`
	Items       []SeededEvent `json:"items"`
}

type ResetResult struct {
	EventsDeleted int64 `json:"events_deleted"`
	UsersDeleted  int64 `json:"users_deleted"`
}

func (s *Service) Overview(ctx context.Context, userID uuid.UUID) (*Overview, error) {
	user, err := s.requireAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.store.AdminOverview(ctx)
	if err != nil {
		return nil, err
	}
	seedEvents, err := s.store.CountSeedEvents(ctx, db.CountSeedEventsParams{
		OrganizerID: user.ID, Slug: seedEventPattern,
	})
	if err != nil {
		return nil, err
	}
	seedUsers, err := s.store.CountSeedUsers(ctx, seedUserEmailPattern)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListSeedEvents(ctx, db.ListSeedEventsParams{
		OrganizerID: user.ID, Slug: seedEventPattern,
	})
	if err != nil {
		return nil, err
	}
	events := make([]SeededEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, SeededEvent{
			ID: row.ID, Slug: row.Slug, Title: row.Title, Category: row.Category,
			ParticipationFormat: row.ParticipationFormat, Status: row.Status,
			IsRemote: row.IsRemote, Country: row.Country, State: row.State, City: row.City,
			RegistrationOpensAt: row.RegistrationOpensAt, RegistrationClosesAt: row.RegistrationClosesAt,
			StartsAt: row.StartsAt, EndsAt: row.EndsAt, SubmissionDeadline: row.SubmissionDeadline,
			JudgingStatus: row.JudgingStatus, ResultsPublishedAt: row.ResultsPublishedAt,
			CreatedAt: row.CreatedAt,
		})
	}
	return &Overview{
		Stats: Stats{
			Users: totals.Users, Events: totals.Events, Submissions: totals.Submissions,
			Judgments: totals.Judgments, Uploads: totals.Uploads,
		},
		SeedEvents: seedEvents, SeedUsers: seedUsers, Events: events,
		SeedEnabled: s.cfg.Env != "production",
	}, nil
}

func (s *Service) Seed(ctx context.Context, userID uuid.UUID, opts SeedOptions) (*SeedResult, error) {
	if s.cfg.Env == "production" {
		return nil, apperr.ErrNotFound
	}
	user, err := s.requireAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	if opts.Events < 0 {
		return nil, apperr.Invalid("events", "Event count cannot be negative.")
	}
	templates := seedTemplates
	if opts.Events > 0 {
		if opts.Events > len(templates) {
			opts.Events = len(templates)
		}
		templates = templates[:opts.Events]
	}
	if opts.Reset {
		if _, err := s.reset(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	hash, err := auth.HashPassword(SeedPassword)
	if err != nil {
		return nil, err
	}
	participants, err := s.ensurePeople(ctx, seedParticipants, hash)
	if err != nil {
		return nil, err
	}
	judges, err := s.ensurePeople(ctx, seedJudges, hash)
	if err != nil {
		return nil, err
	}

	// A full UUID gives enough entropy that additive runs do not collide on
	// the unique event slug.
	run := randomHex(12)
	result := &SeedResult{Users: len(participants) + len(judges)}
	for _, t := range templates {
		submissions, judgments, err := s.seedEvent(ctx, *user, t, participants, judges, run)
		if err != nil {
			return nil, fmt.Errorf("seed %s: %w", t.Key, err)
		}
		result.Events++
		result.Submissions += submissions
		result.Judgments += judgments
	}
	overview, err := s.Overview(ctx, userID)
	if err != nil {
		return nil, err
	}
	result.Items = overview.Events
	return result, nil
}

func (s *Service) Reset(ctx context.Context, userID uuid.UUID) (*ResetResult, error) {
	if s.cfg.Env == "production" {
		return nil, apperr.ErrNotFound
	}
	user, err := s.requireAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.reset(ctx, user.ID)
}

func (s *Service) reset(ctx context.Context, organizerID uuid.UUID) (*ResetResult, error) {
	events, err := s.store.DeleteSeedEvents(ctx, db.DeleteSeedEventsParams{
		OrganizerID: organizerID, Slug: seedEventPattern,
	})
	if err != nil {
		return nil, err
	}
	users, err := s.store.DeleteSeedUsers(ctx, seedUserEmailPattern)
	if err != nil {
		return nil, err
	}
	return &ResetResult{EventsDeleted: events, UsersDeleted: users}, nil
}

func (s *Service) requireAdmin(ctx context.Context, userID uuid.UUID) (*db.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if !user.IsAdmin && !s.cfg.IsAdminEmail(user.Email) {
		return nil, apperr.ErrForbidden
	}
	return &user, nil
}

func (s *Service) ensurePeople(ctx context.Context, people []seedPerson, hash string) ([]db.User, error) {
	out := make([]db.User, 0, len(people))
	for _, p := range people {
		user, err := s.store.GetUserByEmail(ctx, p.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			created, err := s.store.CreateUser(ctx, db.CreateUserParams{Email: p.Email, Username: p.Username})
			if err != nil {
				return nil, err
			}
			user = created
			if err := s.store.CreateIdentity(ctx, db.CreateIdentityParams{
				UserID: user.ID, Provider: "password", ProviderUserID: p.Email, SecretHash: &hash,
			}); err != nil {
				return nil, err
			}
			year := int16(3)
			if _, err := s.store.UpsertProfile(ctx, db.UpsertProfileParams{
				UserID: user.ID, FullName: p.Name, College: p.College, Course: p.Course,
				YearOfStudy: &year, Country: "India",
			}); err != nil {
				return nil, err
			}
			intents, _ := json.Marshal([]string{"find", "judge"})
			if err := s.store.UpdateOnboarding(ctx, db.UpdateOnboardingParams{
				ID: user.ID, OnboardingStep: 6, OnboardingCompleted: true, Intents: intents,
			}); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		out = append(out, user)
	}
	return out, nil
}

func (s *Service) seedEvent(
	ctx context.Context, organizer db.User, t seedTemplate,
	participants, judges []db.User, run string,
) (int, int, error) {
	times := timelineFor(t.Timeline, time.Now())
	location := "Remote / Online"
	country, state, city := "", "", ""
	if !t.Remote {
		country, state, city = t.Country, t.State, t.City
		location = joinLocation(city, state, country)
	}
	criteria, _ := json.Marshal([]map[string]string{
		{"name": "Originality", "description": "Is the idea meaningfully new?"},
		{"name": "Craft", "description": "How considered is the execution?"},
		{"name": "Usefulness", "description": "Would someone actually use this?"},
	})
	row, err := s.store.CreateEvent(ctx, db.CreateEventParams{
		Slug:  fmt.Sprintf("%s%s-%s", seedEventPrefix, t.Key, run),
		Title: t.Title, Description: t.Description, Category: t.Category,
		OrganizerID:         organizer.ID,
		ParticipationFormat: t.Format, Eligibility: t.Eligibility, Rules: t.Rules,
		ContactEmail: organizer.Email, Location: location,
		MinTeamSize: t.MinTeam, MaxTeamSize: t.MaxTeam,
		RegistrationOpensAt: &times.opens, RegistrationClosesAt: &times.closes,
		StartsAt: &times.starts, EndsAt: &times.ends, SubmissionDeadline: &times.deadline,
		Criteria: criteria, IsRemote: t.Remote, Country: country, State: state, City: city,
	})
	if err != nil {
		return 0, 0, err
	}
	if t.Workflow == "draft" {
		return 0, 0, nil
	}
	status := "published"
	if t.Workflow == "closed" {
		status = "closed"
	}
	if err := s.store.UpdateEventStatus(ctx, db.UpdateEventStatusParams{ID: row.ID, Status: status}); err != nil {
		return 0, 0, err
	}
	if err := s.createForms(ctx, row.ID); err != nil {
		return 0, 0, err
	}
	judgeIDs, err := s.inviteJudges(ctx, organizer.ID, row.ID, judges, t.Judges)
	if err != nil {
		return 0, 0, err
	}
	submissionIDs, err := s.createSubmissions(ctx, row.ID, t, participants)
	if err != nil {
		return 0, 0, err
	}
	quality := map[uuid.UUID]int{}
	for i, id := range submissionIDs {
		quality[id] = len(submissionIDs) - i
	}
	judgments := 0
	if t.Workflow == "live" || t.Workflow == "results" {
		judgments, err = s.runJudging(ctx, organizer.ID, row.ID, judgeIDs, quality, t.Workflow)
		if err != nil {
			return len(submissionIDs), judgments, err
		}
	}
	return len(submissionIDs), judgments, nil
}

func (s *Service) createForms(ctx context.Context, eventID uuid.UUID) error {
	fields := []struct {
		Label    string
		Type     string
		Required bool
		Help     string
		Form     string
		Options  []string
	}{
		{"Full name", "text", true, "As it should appear on certificates.", "registration", nil},
		{"Experience level", "select", true, "", "registration", []string{"First event", "A few", "Veteran"}},
		{"Project summary", "long_text", false, "A sentence or two on what you built.", "submission", nil},
	}
	for i, f := range fields {
		raw, _ := json.Marshal(f.Options)
		if _, err := s.store.CreateFormField(ctx, db.CreateFormFieldParams{
			EventID: eventID, Label: f.Label, FieldType: f.Type, Required: f.Required,
			Options: raw, HelpText: f.Help, Position: int32(i + 1), FormKind: f.Form,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) inviteJudges(ctx context.Context, organizerID, eventID uuid.UUID, judges []db.User, count int) ([]uuid.UUID, error) {
	if count > len(judges) {
		count = len(judges)
	}
	out := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		judge := judges[i]
		token := uuid.NewString()
		staff, err := s.store.CreateStaffInvite(ctx, db.CreateStaffInviteParams{
			EventID: eventID, UserID: &judge.ID, Email: judge.Email, Role: "judge",
			InviteToken: &token, InvitedBy: organizerID,
		})
		if err != nil {
			return nil, err
		}
		if err := s.store.AcceptStaffInvite(ctx, db.AcceptStaffInviteParams{
			ID: staff.ID, UserID: &judge.ID,
		}); err != nil {
			return nil, err
		}
		out = append(out, judge.ID)
	}
	return out, nil
}

func (s *Service) createSubmissions(ctx context.Context, eventID uuid.UUID, t seedTemplate, pool []db.User) ([]uuid.UUID, error) {
	if t.Participants == 0 {
		return nil, nil
	}
	people := pickPeople(pool, t.Offset, t.Participants)
	ids := make([]uuid.UUID, 0, t.Participants)
	if t.Format == "team" {
		teamSize := 2
		for start := 0; start < len(people); start += teamSize {
			end := start + teamSize
			if end > len(people) {
				end = len(people)
			}
			members := people[start:end]
			team, err := s.store.CreateTeam(ctx, db.CreateTeamParams{
				EventID: eventID, Name: fmt.Sprintf("%s Team %d", shortTitle(t.Title), start/teamSize+1),
				OwnerID: members[0].ID,
			})
			if err != nil {
				return nil, err
			}
			var creatorRegID *uuid.UUID
			for memberIndex, member := range members {
				if err := s.store.AddTeamMember(ctx, db.AddTeamMemberParams{
					EventID: eventID, TeamID: team.ID, UserID: member.ID,
				}); err != nil {
					return nil, err
				}
				answers, _ := json.Marshal(map[string]string{
					"full_name": member.Username, "experience_level": "A few",
				})
				reg, err := s.store.CreateRegistrationForTeamMember(ctx, db.CreateRegistrationForTeamMemberParams{
					EventID: eventID, UserID: member.ID, TeamID: &team.ID, FormAnswers: answers,
				})
				if err != nil {
					return nil, err
				}
				if memberIndex == 0 {
					creatorRegID = &reg.ID
				}
			}
			sub, err := s.createSubmission(ctx, eventID, members[0].ID, &team.ID, creatorRegID, t, start/teamSize)
			if err != nil {
				return nil, err
			}
			ids = append(ids, sub.ID)
		}
		return ids, nil
	}
	for i, person := range people {
		answers, _ := json.Marshal(map[string]string{
			"full_name": person.Username, "experience_level": "A few",
		})
		reg, err := s.store.CreateRegistrationForTeamMember(ctx, db.CreateRegistrationForTeamMemberParams{
			EventID: eventID, UserID: person.ID, FormAnswers: answers,
		})
		if err != nil {
			return nil, err
		}
		sub, err := s.createSubmission(ctx, eventID, person.ID, nil, &reg.ID, t, i)
		if err != nil {
			return nil, err
		}
		ids = append(ids, sub.ID)
	}
	return ids, nil
}

func (s *Service) createSubmission(
	ctx context.Context, eventID, creatorID uuid.UUID, teamID, regID *uuid.UUID,
	t seedTemplate, position int,
) (db.Submission, error) {
	answers, _ := json.Marshal(map[string]string{"project_summary": "A small, useful tool built for the event."})
	return s.store.CreateSubmission(ctx, db.CreateSubmissionParams{
		EventID: eventID, RegistrationID: regID, TeamID: teamID, CreatedBy: creatorID,
		Title:       fmt.Sprintf("%s Entry %d", shortTitle(t.Title), position+1),
		Description: "A seeded project used to exercise judging and results.",
		RepoUrl:     fmt.Sprintf("https://example.com/seed/%s/%d/repo", t.Key, position+1),
		DemoUrl:     fmt.Sprintf("https://example.com/seed/%s/%d/demo", t.Key, position+1),
		Status:      "submitted", Answers: answers,
	})
}

func (s *Service) runJudging(
	ctx context.Context, organizerID, eventID uuid.UUID, judgeIDs []uuid.UUID,
	quality map[uuid.UUID]int, workflow string,
) (int, error) {
	if len(judgeIDs) == 0 {
		return 0, nil
	}
	if _, err := s.judging.Assign(ctx, organizerID, eventID); err != nil {
		return 0, err
	}
	if err := s.judging.Start(ctx, organizerID, eventID); err != nil {
		return 0, err
	}
	judgments := 0
	for i, judgeID := range judgeIDs {
		limit := 1 << 30
		if workflow == "live" {
			// Leave a visible backlog: only the first judge completes a few pairs.
			if i > 0 {
				limit = 0
			} else {
				limit = 3
			}
		}
		for n := 0; n < limit; n++ {
			next, err := s.judging.Next(ctx, judgeID, eventID)
			if err != nil {
				return judgments, err
			}
			if next.Pair == nil {
				break
			}
			winner := "a"
			if quality[next.Pair.B.ID] > quality[next.Pair.A.ID] {
				winner = "b"
			}
			if err := s.judging.Compare(ctx, judgeID, eventID, next.Pair.ID, winner, false, false); err != nil {
				return judgments, err
			}
			judgments++
		}
	}
	if workflow == "results" {
		if _, err := s.judging.Publish(ctx, organizerID, eventID, false); err != nil {
			return judgments, err
		}
	}
	return judgments, nil
}

type seedPerson struct {
	Email    string
	Username string
	Name     string
	College  string
	Course   string
}

var seedParticipants = []seedPerson{
	{"seed.user01@compytition.local", "seed_user_01", "Aarav Mehta", "IIT Bombay", "Computer Science"},
	{"seed.user02@compytition.local", "seed_user_02", "Diya Nair", "VIT Vellore", "Information Technology"},
	{"seed.user03@compytition.local", "seed_user_03", "Kabir Rao", "NIT Trichy", "Electronics"},
	{"seed.user04@compytition.local", "seed_user_04", "Meera Iyer", "BITS Pilani", "Design"},
	{"seed.user05@compytition.local", "seed_user_05", "Rohan Das", "IIIT Hyderabad", "Data Science"},
	{"seed.user06@compytition.local", "seed_user_06", "Sana Khan", "IIT Delhi", "Computer Science"},
	{"seed.user07@compytition.local", "seed_user_07", "Vikram Bose", "Jadavpur University", "Mechanical"},
	{"seed.user08@compytition.local", "seed_user_08", "Isha Kulkarni", "COEP Pune", "Computer Science"},
	{"seed.user09@compytition.local", "seed_user_09", "Neel Shah", "NID Ahmedabad", "Interaction Design"},
	{"seed.user10@compytition.local", "seed_user_10", "Tara Menon", "IIIT Bangalore", "Computer Science"},
	{"seed.user11@compytition.local", "seed_user_11", "Arjun Pillai", "NIT Surathkal", "Information Technology"},
	{"seed.user12@compytition.local", "seed_user_12", "Zoya Ahmed", "Jamia Millia Islamia", "Architecture"},
	{"seed.user13@compytition.local", "seed_user_13", "Dev Kapoor", "DTU Delhi", "Software Engineering"},
	{"seed.user14@compytition.local", "seed_user_14", "Lina Fernandes", "Goa University", "Computer Science"},
	{"seed.user15@compytition.local", "seed_user_15", "Manav Joshi", "IIT Kanpur", "Electrical"},
	{"seed.user16@compytition.local", "seed_user_16", "Riya Sen", "IIM Bangalore", "Product Management"},
}

var seedJudges = []seedPerson{
	{"seed.judge1@compytition.local", "seed_judge_1", "Nisha Rao", "IISc Bangalore", "Computer Science"},
	{"seed.judge2@compytition.local", "seed_judge_2", "Omar Sheikh", "IIT Madras", "Human Computer Interaction"},
	{"seed.judge3@compytition.local", "seed_judge_3", "Lena Fischer", "TU Berlin", "Robotics"},
	{"seed.judge4@compytition.local", "seed_judge_4", "Kenji Tan", "NUS", "Design"},
}

type seedTemplate struct {
	Key          string
	Title        string
	Description  string
	Category     string
	Format       string
	MinTeam      int16
	MaxTeam      int16
	Remote       bool
	Country      string
	State        string
	City         string
	Timeline     string
	Participants int
	Judges       int
	Workflow     string
	Offset       int
	Eligibility  string
	Rules        string
}

var seedTemplates = []seedTemplate{
	{
		Key: "mumbai-metro-hack", Title: "Mumbai Metro Hack",
		Description: "A weekend hackathon for tools that make city travel calmer and faster.",
		Category:    "Hackathon", Format: "both", MinTeam: 1, MaxTeam: 4,
		Country: "India", State: "Maharashtra", City: "Mumbai",
		Timeline: "open", Participants: 6, Judges: 3, Workflow: "open",
		Eligibility: "Open to students and recent graduates.", Rules: "Teams of up to four. Start fresh.",
	},
	{
		Key: "bengaluru-data-sprint", Title: "Bengaluru Data Sprint",
		Description: "A one-day sprint on open civic data, judged by a single domain expert.",
		Category:    "Competition", Format: "individual", MinTeam: 1, MaxTeam: 1,
		Country: "India", State: "Karnataka", City: "Bengaluru",
		Timeline: "ended", Participants: 5, Judges: 1, Workflow: "results", Offset: 3,
		Eligibility: "Solo participants only.", Rules: "Use the provided open datasets.",
	},
	{
		Key: "global-design-jam", Title: "Global Design Jam",
		Description: "A remote design jam for accessible interfaces and calm onboarding flows.",
		Category:    "Workshop", Format: "individual", MinTeam: 1, MaxTeam: 1,
		Remote:   true,
		Timeline: "closing", Participants: 4, Judges: 0, Workflow: "open", Offset: 7,
		Eligibility: "Anyone with a browser.", Rules: "Submit a short walkthrough and a prototype link.",
	},
	{
		Key: "berlin-robotics-challenge", Title: "Berlin Robotics Challenge",
		Description: "Two-person teams build a tabletop robot that can sort mixed recycling.",
		Category:    "Contest", Format: "team", MinTeam: 2, MaxTeam: 2,
		Country: "Germany", State: "Berlin", City: "Berlin",
		Timeline: "waiting", Participants: 8, Judges: 3, Workflow: "live", Offset: 1,
		Eligibility: "Teams of exactly two.", Rules: "Kit robots only. Safety review required.",
	},
	{
		Key: "london-game-jam", Title: "London Game Jam",
		Description: "A 48-hour game jam with a theme reveal at the opening keynote.",
		Category:    "Hackathon", Format: "team", MinTeam: 1, MaxTeam: 4,
		Country: "United Kingdom", State: "England", City: "London",
		Timeline: "upcoming", Participants: 0, Judges: 2, Workflow: "published",
		Eligibility: "Open to all experience levels.", Rules: "Any engine. Assets must be licensed.",
	},
	{
		Key: "singapore-fintech-ideathon", Title: "Singapore FinTech Ideathon",
		Description: "A closed-registration ideathon on inclusive payments and savings tools.",
		Category:    "Competition", Format: "both", MinTeam: 1, MaxTeam: 3,
		Country: "Singapore", State: "Singapore", City: "Singapore",
		Timeline: "waiting", Participants: 5, Judges: 2, Workflow: "setup", Offset: 5,
		Eligibility: "Students and early-career builders.", Rules: "No live customer data.",
	},
	{
		Key: "remote-research-sprint", Title: "Remote Research Sprint",
		Description: "A short remote sprint on reproducible notebooks and readable papers.",
		Category:    "Workshop", Format: "individual", MinTeam: 1, MaxTeam: 1,
		Remote:   true,
		Timeline: "ended", Participants: 4, Judges: 1, Workflow: "results", Offset: 2,
		Eligibility: "Solo researchers and students.", Rules: "Share a public notebook.",
	},
	{
		Key: "austin-ai-hack", Title: "Austin AI Hack",
		Description: "Small teams ship an AI assistant that explains its own reasoning.",
		Category:    "Hackathon", Format: "team", MinTeam: 2, MaxTeam: 3,
		Country: "United States", State: "Texas", City: "Austin",
		Timeline: "ended", Participants: 6, Judges: 3, Workflow: "results", Offset: 9,
		Eligibility: "Teams of two or three.", Rules: "Open models preferred. No hidden prompts.",
	},
	{
		Key: "toronto-climate-hack", Title: "Toronto Climate Hack",
		Description: "A climate-focused hack for neighbourhood heat and transit resilience.",
		Category:    "Contest", Format: "both", MinTeam: 1, MaxTeam: 4,
		Country: "Canada", State: "Ontario", City: "Toronto",
		Timeline: "upcoming", Participants: 0, Judges: 0, Workflow: "published",
		Eligibility: "Open to the public.", Rules: "Use open climate data where possible.",
	},
	{
		Key: "online-civic-tech", Title: "Online Civic Tech Sprint",
		Description: "A remote sprint for tools that make local government easier to reach.",
		Category:    "Other", Format: "both", MinTeam: 1, MaxTeam: 3,
		Remote:   true,
		Timeline: "waiting", Participants: 5, Judges: 2, Workflow: "closed", Offset: 11,
		Eligibility: "Anyone interested in civic tech.", Rules: "Respect privacy and accessibility.",
	},
}

type seedTimeline struct {
	opens, closes, starts, ends, deadline time.Time
}

func timelineFor(kind string, now time.Time) seedTimeline {
	day := 24 * time.Hour
	switch kind {
	case "draft", "upcoming":
		return seedTimeline{
			opens: now.Add(7 * day), closes: now.Add(21 * day),
			starts: now.Add(30 * day), ends: now.Add(32 * day), deadline: now.Add(31 * day),
		}
	case "open":
		return seedTimeline{
			opens: now.Add(-7 * day), closes: now.Add(14 * day),
			starts: now.Add(20 * day), ends: now.Add(22 * day), deadline: now.Add(21 * day),
		}
	case "closing":
		return seedTimeline{
			opens: now.Add(-20 * day), closes: now.Add(2 * day),
			starts: now.Add(5 * day), ends: now.Add(7 * day), deadline: now.Add(6 * day),
		}
	case "waiting":
		return seedTimeline{
			opens: now.Add(-30 * day), closes: now.Add(-10 * day),
			starts: now.Add(-5 * day), ends: now.Add(4 * day), deadline: now.Add(-2 * day),
		}
	default: // ended
		return seedTimeline{
			opens: now.Add(-45 * day), closes: now.Add(-25 * day),
			starts: now.Add(-18 * day), ends: now.Add(-8 * day), deadline: now.Add(-10 * day),
		}
	}
}

func pickPeople(pool []db.User, offset, count int) []db.User {
	if len(pool) == 0 || count <= 0 {
		return nil
	}
	if count > len(pool) {
		count = len(pool)
	}
	out := make([]db.User, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, pool[(offset+i)%len(pool)])
	}
	return out
}

func joinLocation(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	return strings.Join(nonEmpty, ", ")
}

func shortTitle(title string) string {
	words := strings.Fields(title)
	if len(words) <= 2 {
		return title
	}
	return strings.Join(words[:2], " ")
}

func randomHex(n int) string {
	id := uuid.NewString()
	clean := strings.ReplaceAll(id, "-", "")
	if n > len(clean) {
		n = len(clean)
	}
	return clean[:n]
}
