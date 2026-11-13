// Command seed loads a deterministic demo dataset: one organizer, three judges,
// six participants, a published hackathon, six submissions, simulated pairwise
// judgments and published results. It is idempotent: running it twice does nothing.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"compytition/internal/auth"
	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/judging"
)

const (
	password = "demo1234"
	slug     = "demo-hack-2026"
)

type person struct {
	Email    string
	Username string
	Name     string
	College  string
	Course   string
	Team     string
	Title    string
	Pitch    string
}

var people = []person{
	{"alice@compytition.local", "alice", "Alice Menon", "IIT Bombay", "Computer Science", "Byte Benders", "Loopback", "A zero-config debugger for distributed Go services."},
	{"bob@compytition.local", "bob", "Bob Iyer", "IIT Bombay", "Computer Science", "Byte Benders", "Relay", "A local-first publish-subscribe relay for small teams."},
	{"carol@compytition.local", "carol", "Carol Dsouza", "VIT Vellore", "Information Technology", "", "Hearth", "A calm co-working timer that protects deep work blocks."},
	{"dave@compytition.local", "dave", "Dave Kumar", "NIT Trichy", "Electronics", "", "Orbit", "A tiny satellite tracker that works offline."},
	{"erin@compytition.local", "erin", "Erin Shah", "BITS Pilani", "Design", "", "Paper Trail", "Turn a folder of PDFs into a searchable reading queue."},
	{"frank@compytition.local", "frank", "Frank Rao", "IIIT Hyderabad", "Data Science", "", "Mosaic", "Accessible color palettes generated from a single photo."},
}

var judges = []person{
	{"ava@compytition.local", "judge_ava", "Ava Nair", "IIT Delhi", "Computer Science", "", "", ""},
	{"milo@compytition.local", "judge_milo", "Milo Fernandes", "NIT Surathkal", "Design", "", "", ""},
	{"rin@compytition.local", "judge_rin", "Rin Kapoor", "IIIT Bangalore", "Humanities", "", "", ""},
}

func main() {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		slog.Error("refusing to run the demo seed with APP_ENV=production")
		os.Exit(1)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store, err := db.NewStore(ctx, url, 5)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	hash, err := auth.HashPassword(password)
	if err != nil {
		slog.Error("hash password", "error", err)
		os.Exit(1)
	}

	admin := ensureUser(ctx, store, hash, person{
		Email: "admin@compytition.local", Username: "admin_user",
		Name: "Admin User", College: "Compytition", Course: "Operations",
	})
	if err := store.MakeUserAdmin(ctx, admin.ID); err != nil {
		slog.Error("make user admin", "error", err)
		os.Exit(1)
	}

	if _, err := store.GetEventBySlug(ctx, slug); err == nil {
		slog.Info("seed already applied", "slug", slug)
		fmt.Printf("seed ready\n  admin:     admin@compytition.local / %s\n", password)
		return
	}

	organizer := mustUser(ctx, store, hash, person{
		Email: "host@compytition.local", Username: "demo_host",
		Name: "Priya Sharma", College: "IIT Bombay", Course: "Computer Science",
	})
	judgeIDs := make([]uuid.UUID, 0, len(judges))
	for _, j := range judges {
		judgeIDs = append(judgeIDs, mustUser(ctx, store, hash, j).ID)
	}
	participants := make([]db.User, 0, len(people))
	for _, p := range people {
		participants = append(participants, *mustUser(ctx, store, hash, p))
	}

	now := time.Now()
	opens := now.Add(-14 * 24 * time.Hour)
	closes := now.Add(-10 * 24 * time.Hour)
	starts := now.Add(-7 * 24 * time.Hour)
	ends := now.Add(-2 * 24 * time.Hour)
	deadline := now.Add(-3 * 24 * time.Hour)
	criteria, _ := json.Marshal([]map[string]string{
		{"name": "Originality", "description": "Is the idea meaningfully new?"},
		{"name": "Craft", "description": "Is the implementation or design considered?"},
		{"name": "Usefulness", "description": "Would someone actually use this?"},
	})
	ev, err := store.CreateEvent(ctx, db.CreateEventParams{
		Slug: slug, OrganizerID: organizer.ID, Title: "Demo Hack 2026",
		Description: "A weekend build sprint for small, useful tools. Ship something you would use on Monday.",
		Category:    "Hackathon", ParticipationFormat: "both",
		Eligibility:  "Open to students and recent graduates.",
		Rules:        "Teams of up to four. Work must be started during the event.",
		ContactEmail: "host@compytition.local", Location: "Online",
		IsRemote: true, MergeRegistrationSubmission: false,
		MinTeamSize: 1, MaxTeamSize: 4,
		RegistrationOpensAt: &opens, RegistrationClosesAt: &closes,
		StartsAt: &starts, EndsAt: &ends, SubmissionDeadline: &deadline,
		Criteria: criteria,
	})
	if err != nil {
		slog.Error("create event", "error", err)
		os.Exit(1)
	}
	if err := store.UpdateEventStatus(ctx, db.UpdateEventStatusParams{ID: ev.ID, Status: "published"}); err != nil {
		slog.Error("publish event", "error", err)
		os.Exit(1)
	}

	fields := []struct {
		Label    string
		Type     string
		Required bool
		Help     string
		Options  []string
	}{
		{"Full name", "text", true, "As it should appear on certificates.", nil},
		{"College or company", "text", true, "", nil},
		{"What are you hoping to build?", "long_text", false, "A sentence or two is plenty.", nil},
		{"Experience level", "select", true, "", []string{"First hackathon", "A few", "Veteran"}},
	}
	for i, f := range fields {
		raw, _ := json.Marshal(f.Options)
		if _, err := store.CreateFormField(ctx, db.CreateFormFieldParams{
			EventID: ev.ID, Label: f.Label, FieldType: f.Type, Required: f.Required,
			Options: raw, HelpText: f.Help, Position: int32(i + 1), FormKind: "registration",
		}); err != nil {
			slog.Error("create form field", "error", err)
			os.Exit(1)
		}
	}

	for _, id := range judgeIDs {
		u, err := store.GetUserByID(ctx, id)
		if err != nil {
			continue
		}
		token := uuid.NewString()
		staff, err := store.CreateStaffInvite(ctx, db.CreateStaffInviteParams{
			EventID: ev.ID, UserID: &id, Email: u.Email, Role: "judge",
			InviteToken: &token, InvitedBy: organizer.ID,
		})
		if err != nil {
			slog.Error("invite judge", "error", err)
			os.Exit(1)
		}
		if err := store.AcceptStaffInvite(ctx, db.AcceptStaffInviteParams{ID: staff.ID, UserID: &id}); err != nil {
			slog.Error("accept judge", "error", err)
			os.Exit(1)
		}
	}

	titles := map[uuid.UUID]string{}
	for i := range participants {
		user := participants[i]
		person := people[i]
		answers, _ := json.Marshal(map[string]string{
			"full_name": person.Name, "college": person.College, "email": person.Email,
		})
		reg, err := store.CreateRegistrationForTeamMember(ctx, db.CreateRegistrationForTeamMemberParams{
			EventID: ev.ID, UserID: user.ID, FormAnswers: answers,
		})
		if err != nil {
			slog.Error("create registration", "error", err)
			os.Exit(1)
		}
		sub, err := store.CreateSubmission(ctx, db.CreateSubmissionParams{
			EventID: ev.ID, RegistrationID: &reg.ID, CreatedBy: user.ID,
			Title: person.Title, Description: person.Pitch, Status: "submitted",
			RepoUrl: "https://example.com/" + person.Username + "/repo",
			DemoUrl: "https://example.com/" + person.Username + "/demo",
			Answers: []byte(`{}`),
		})
		if err != nil {
			slog.Error("create submission", "error", err)
			os.Exit(1)
		}
		titles[sub.ID] = person.Title
	}

	if err := finalizeDemoJudging(ctx, store, organizer.ID, ev.ID, judgeIDs, titles); err != nil {
		slog.Error("seed judging", "error", err)
		os.Exit(1)
	}

	fmt.Printf("seed ready\n  admin:     admin@compytition.local / %s\n  organizer: host@compytition.local / %s\n  judges:    judge_ava, judge_milo, judge_rin / %s\n  event:     /events/%s\n", password, password, password, ev.ID)
}

func mustUser(ctx context.Context, store *db.Store, hash string, p person) *db.User {
	user, err := store.CreateUser(ctx, db.CreateUserParams{Email: p.Email, Username: p.Username})
	if err != nil {
		slog.Error("create user", "email", p.Email, "error", err)
		os.Exit(1)
	}
	finishUser(ctx, store, hash, user, p)
	return &user
}

// ensureUser is idempotent so the admin account exists even when the demo
// event was seeded by an earlier run. It only promotes an existing account when
// that account can prove ownership with the seed password; otherwise an
// attacker who registered the admin email first would be granted privileges.
func ensureUser(ctx context.Context, store *db.Store, hash string, p person) *db.User {
	user, err := store.GetUserByEmail(ctx, p.Email)
	if err == nil {
		identity, idErr := store.GetIdentity(ctx, db.GetIdentityParams{Provider: "password", ProviderUserID: p.Email})
		switch {
		case errors.Is(idErr, pgx.ErrNoRows):
			slog.Error("refusing to promote an existing account without a seed password", "email", p.Email)
			os.Exit(1)
		case idErr != nil:
			slog.Error("look up identity", "email", p.Email, "error", idErr)
			os.Exit(1)
		case identity.SecretHash == nil:
			slog.Error("refusing to promote an existing account without a password", "email", p.Email)
			os.Exit(1)
		default:
			ok, verifyErr := auth.VerifyPassword(password, *identity.SecretHash)
			if verifyErr != nil || !ok {
				slog.Error("refusing to promote an existing account whose password is not the seed password", "email", p.Email)
				os.Exit(1)
			}
		}
		return &user
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("look up user", "email", p.Email, "error", err)
		os.Exit(1)
	}
	return mustUser(ctx, store, hash, p)
}

func finishUser(ctx context.Context, store *db.Store, hash string, user db.User, p person) {
	if err := store.CreateIdentity(ctx, db.CreateIdentityParams{
		UserID: user.ID, Provider: "password", ProviderUserID: p.Email, SecretHash: &hash,
	}); err != nil {
		slog.Error("create identity", "email", p.Email, "error", err)
		os.Exit(1)
	}
	year := int16(3)
	if _, err := store.UpsertProfile(ctx, db.UpsertProfileParams{
		UserID: user.ID, FullName: p.Name, College: p.College, Course: p.Course,
		YearOfStudy: &year,
	}); err != nil {
		slog.Error("create profile", "email", p.Email, "error", err)
		os.Exit(1)
	}
	intents, _ := json.Marshal([]string{"find", "host", "judge"})
	if err := store.UpdateOnboarding(ctx, db.UpdateOnboardingParams{
		ID: user.ID, OnboardingStep: 6, OnboardingCompleted: true, Intents: intents,
	}); err != nil {
		slog.Error("complete onboarding", "email", p.Email, "error", err)
		os.Exit(1)
	}
}

// finalizeDemoJudging runs the real judging pipeline against the seed data so
// the demo event ships with pairwise judgments and published results.
func finalizeDemoJudging(ctx context.Context, store *db.Store, organizerID, eventID uuid.UUID, judgeIDs []uuid.UUID, titles map[uuid.UUID]string) error {
	svc := judging.NewService(store, &config.Config{}, slog.Default())
	if _, err := svc.Assign(ctx, organizerID, eventID); err != nil {
		return fmt.Errorf("assign comparisons: %w", err)
	}
	if err := svc.Start(ctx, organizerID, eventID); err != nil {
		return fmt.Errorf("start judging: %w", err)
	}

	quality := map[string]int{
		"Loopback":    6,
		"Relay":       5,
		"Paper Trail": 4,
		"Mosaic":      3,
		"Hearth":      2,
		"Orbit":       1,
	}
	for _, judgeID := range judgeIDs {
		for i := 0; i < 500; i++ {
			next, err := svc.Next(ctx, judgeID, eventID)
			if err != nil {
				return fmt.Errorf("next pair: %w", err)
			}
			if next.Pair == nil {
				break
			}
			winner := "a"
			if quality[titles[next.Pair.B.ID]] > quality[titles[next.Pair.A.ID]] {
				winner = "b"
			}
			if err := svc.Compare(ctx, judgeID, eventID, next.Pair.ID, winner, false, false); err != nil {
				return fmt.Errorf("record comparison: %w", err)
			}
		}
	}

	if _, err := svc.Publish(ctx, organizerID, eventID, false); err != nil {
		return fmt.Errorf("publish results: %w", err)
	}
	return nil
}
