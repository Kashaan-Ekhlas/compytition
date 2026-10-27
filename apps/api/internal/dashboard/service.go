// Package dashboard assembles the user's home screen in one request: what they
// registered for, their teams and pending invites, their submissions, and the
// events they host or judge. It is read-only and deliberately returns compact
// slices instead of paginated lists.
package dashboard

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/events"
	"compytition/internal/registrations"
	"compytition/internal/submissions"
	"compytition/internal/teams"
)

type Service struct {
	store         *db.Store
	events        *events.Service
	registrations *registrations.Service
	teams         *teams.Service
	submissions   *submissions.Service
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{
		store:         store,
		events:        events.NewService(store, cfg, log),
		registrations: registrations.NewService(store, cfg, log),
		teams:         teams.NewService(store, cfg, log),
		submissions:   submissions.NewService(store, cfg, log),
	}
}

type Response struct {
	Registrations       []registrations.View    `json:"registrations"`
	Teams               []teams.TeamView        `json:"teams"`
	TeamInvites         []teams.InviteView      `json:"team_invites"`
	JoinRequests        []teams.JoinRequestView `json:"join_requests"`
	Submissions         []submissions.View      `json:"submissions"`
	HostedEvents        []events.EventView      `json:"hosted_events"`
	JudgeEvents         []events.EventView      `json:"judge_events"`
	UnreadNotifications int32                   `json:"unread_notifications"`
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (*Response, error) {
	out := &Response{
		Registrations: []registrations.View{},
		Teams:         []teams.TeamView{},
		TeamInvites:   []teams.InviteView{},
		JoinRequests:  []teams.JoinRequestView{},
		Submissions:   []submissions.View{},
		HostedEvents:  []events.EventView{},
		JudgeEvents:   []events.EventView{},
	}

	if list, err := s.registrations.ListMine(ctx, userID); err == nil {
		out.Registrations = list
	}
	if list, err := s.teams.ListMine(ctx, userID); err == nil {
		out.Teams = list
	}
	if list, err := s.teams.ListMyInvites(ctx, userID); err == nil {
		out.TeamInvites = list
	}
	if list, err := s.teams.ListMyJoinRequests(ctx, userID); err == nil {
		out.JoinRequests = list
	}
	if list, err := s.submissions.ListMine(ctx, userID); err == nil {
		out.Submissions = list
	}
	if list, err := s.events.ListHosted(ctx, userID); err == nil {
		out.HostedEvents = list
	}
	if list, err := s.events.ListForJudge(ctx, userID); err == nil {
		out.JudgeEvents = list
	}
	if count, err := s.store.CountUnreadNotifications(ctx, userID); err == nil {
		out.UnreadNotifications = count
	}

	if out.Registrations == nil {
		out.Registrations = []registrations.View{}
	}
	if out.Teams == nil {
		out.Teams = []teams.TeamView{}
	}
	if out.TeamInvites == nil {
		out.TeamInvites = []teams.InviteView{}
	}
	if out.JoinRequests == nil {
		out.JoinRequests = []teams.JoinRequestView{}
	}
	if out.Submissions == nil {
		out.Submissions = []submissions.View{}
	}
	if out.HostedEvents == nil {
		out.HostedEvents = []events.EventView{}
	}
	if out.JudgeEvents == nil {
		out.JudgeEvents = []events.EventView{}
	}
	return out, nil
}
