package admin

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"compytition/internal/db"
)

// TestSeedTemplatesCoverPublicFilters guards the requirement that the seeded
// set exercises every filter exposed by public event discovery.
func TestSeedTemplatesCoverPublicFilters(t *testing.T) {
	if len(seedTemplates) != 10 {
		t.Fatalf("templates = %d, want 10", len(seedTemplates))
	}
	categories := map[string]bool{}
	formats := map[string]bool{}
	timelines := map[string]bool{}
	workflows := map[string]bool{}
	countries := map[string]bool{}
	remote := false
	onsite := false
	for _, template := range seedTemplates {
		categories[template.Category] = true
		formats[template.Format] = true
		timelines[template.Timeline] = true
		workflows[template.Workflow] = true
		if template.Remote {
			remote = true
			continue
		}
		onsite = true
		if template.Country == "" || template.State == "" || template.City == "" {
			t.Fatalf("template %q is onsite but missing a location", template.Key)
		}
		countries[template.Country] = true
	}
	for name, values := range map[string]map[string]bool{
		"category": categories, "format": formats, "timeline": timelines,
		"workflow": workflows, "country": countries,
	} {
		if len(values) < 3 {
			t.Fatalf("%s covers %d values, want at least 3", name, len(values))
		}
	}
	if !remote || !onsite {
		t.Fatalf("templates must include remote and onsite events, remote=%v onsite=%v", remote, onsite)
	}
	for _, format := range []string{"individual", "team", "both"} {
		if !formats[format] {
			t.Fatalf("templates are missing the %q format", format)
		}
	}
	for _, workflow := range []string{"open", "live", "results", "closed"} {
		if !workflows[workflow] {
			t.Fatalf("templates are missing the %q workflow", workflow)
		}
	}
	if !workflows["setup"] && !workflows["published"] {
		t.Fatal("templates must include a pre-judging or draft-only workflow")
	}
}

func TestTimelineForKeepsDeadlinesOrdered(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	for _, kind := range []string{"draft", "upcoming", "open", "closing", "waiting", "ended"} {
		got := timelineFor(kind, now)
		if !got.opens.Before(got.closes) {
			t.Fatalf("%s: registration opens %v is not before closes %v", kind, got.opens, got.closes)
		}
		if !got.starts.Before(got.ends) {
			t.Fatalf("%s: event starts %v is not before ends %v", kind, got.starts, got.ends)
		}
		if !got.deadline.Before(got.ends) {
			t.Fatalf("%s: submission deadline %v is not before event end %v", kind, got.deadline, got.ends)
		}
	}
}

func TestJoinLocationAndShortTitle(t *testing.T) {
	if got := joinLocation(" Mumbai ", "", "Maharashtra"); got != "Mumbai, Maharashtra" {
		t.Fatalf("joinLocation = %q", got)
	}
	if got := shortTitle("Mumbai Metro Hack"); got != "Mumbai Metro" {
		t.Fatalf("shortTitle = %q", got)
	}
	if got := shortTitle("FinTech"); got != "FinTech" {
		t.Fatalf("shortTitle single word = %q", got)
	}
}

func TestPickPeopleWrapsAroundThePool(t *testing.T) {
	pool := make([]db.User, 4)
	for i := range pool {
		pool[i].ID = uuid.New()
	}
	got := pickPeople(pool, 3, 4)
	if len(got) != 4 {
		t.Fatalf("picked %d people, want 4", len(got))
	}
	if got[0].ID != pool[3].ID || got[1].ID != pool[0].ID {
		t.Fatalf("pickPeople did not wrap around the pool")
	}
	if got := pickPeople(nil, 0, 3); got != nil {
		t.Fatalf("empty pool = %v, want nil", got)
	}
}
