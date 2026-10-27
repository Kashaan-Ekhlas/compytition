package teams

import (
	"context"
	"errors"
	"log/slog"
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

type MemberView struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	JoinedAt time.Time `json:"joined_at"`
	IsOwner  bool      `json:"is_owner"`
}

type InviteView struct {
	ID                uuid.UUID `json:"id"`
	TeamID            uuid.UUID `json:"team_id"`
	InvitedUserID     uuid.UUID `json:"invited_user_id"`
	InvitedBy         uuid.UUID `json:"invited_by"`
	Status            string    `json:"status"`
	TeamName          string    `json:"team_name,omitempty"`
	EventID           uuid.UUID `json:"event_id,omitempty"`
	EventTitle        string    `json:"event_title,omitempty"`
	InvitedByUsername string    `json:"invited_by_username,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type JoinRequestView struct {
	ID         uuid.UUID `json:"id"`
	TeamID     uuid.UUID `json:"team_id"`
	UserID     uuid.UUID `json:"user_id"`
	Message    string    `json:"message"`
	Status     string    `json:"status"`
	Username   string    `json:"username,omitempty"`
	FullName   string    `json:"full_name,omitempty"`
	TeamName   string    `json:"team_name,omitempty"`
	EventID    uuid.UUID `json:"event_id,omitempty"`
	EventTitle string    `json:"event_title,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type TeamView struct {
	ID                  uuid.UUID         `json:"id"`
	EventID             uuid.UUID         `json:"event_id"`
	EventTitle          string            `json:"event_title,omitempty"`
	EventSlug           string            `json:"event_slug,omitempty"`
	EventStatus         string            `json:"event_status,omitempty"`
	Name                string            `json:"name"`
	OwnerID             uuid.UUID         `json:"owner_id"`
	OwnerUsername       string            `json:"owner_username"`
	IsLookingForMembers bool              `json:"is_looking_for_members"`
	MemberCount         int32             `json:"member_count"`
	MaxTeamSize         int16             `json:"max_team_size,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	Members             []MemberView      `json:"members,omitempty"`
	IsMember            bool              `json:"is_member"`
	IsOwner             bool              `json:"is_owner"`
	MyPendingInvite     bool              `json:"my_pending_invite"`
	MyPendingRequest    bool              `json:"my_pending_request"`
	PendingInvites      []InviteView      `json:"pending_invites,omitempty"`
	PendingRequests     []JoinRequestView `json:"pending_requests,omitempty"`
}

type ListOptions struct {
	LookingForMembers bool
}

func (s *Service) Create(ctx context.Context, userID, eventID uuid.UUID, name string) (*TeamView, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := ensureTeamFormationOpen(ev); err != nil {
		return nil, err
	}
	if ev.ParticipationFormat == "individual" {
		return nil, apperr.Invalid("format", "This event is individual-only.")
	}
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 80 {
		return nil, apperr.Invalid("name", "Team name must be 2–80 characters.")
	}
	existing, err := s.store.ListTeamMembershipInEvent(ctx, db.ListTeamMembershipInEventParams{EventID: eventID, UserID: userID})
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, apperr.Invalid("team", "You are already in a team for this event.")
	}
	var team db.Team
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		t, err := q.CreateTeam(ctx, db.CreateTeamParams{EventID: eventID, Name: name, OwnerID: userID})
		if err != nil {
			if unique(err) {
				return apperr.Invalid("name", "A team with that name already exists.")
			}
			return err
		}
		if err := q.AddTeamMember(ctx, db.AddTeamMemberParams{EventID: eventID, TeamID: t.ID, UserID: userID}); err != nil {
			if unique(err) {
				return apperr.Invalid("team", "You are already in a team for this event.")
			}
			return err
		}
		team = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, &userID, team.ID)
}

func (s *Service) ListForEvent(ctx context.Context, viewerID *uuid.UUID, eventID uuid.UUID, opts ListOptions) ([]TeamView, error) {
	rows, err := s.store.ListTeamsForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	var invitedTeams = map[uuid.UUID]bool{}
	var requestedTeams = map[uuid.UUID]bool{}
	var myTeam *uuid.UUID
	if viewerID != nil {
		mine, _ := s.store.GetTeamForEventUser(ctx, db.GetTeamForEventUserParams{EventID: eventID, UserID: *viewerID})
		if mine.ID != uuid.Nil {
			myTeam = &mine.ID
		}
		invites, _ := s.store.ListMyTeamInvites(ctx, *viewerID)
		for _, inv := range invites {
			if inv.EventID == eventID {
				invitedTeams[inv.TeamID] = true
			}
		}
		requests, _ := s.store.ListMyJoinRequests(ctx, *viewerID)
		for _, req := range requests {
			if req.EventID == eventID {
				requestedTeams[req.TeamID] = true
			}
		}
	}
	out := make([]TeamView, 0, len(rows))
	for _, r := range rows {
		if opts.LookingForMembers && !r.IsLookingForMembers {
			continue
		}
		v := TeamView{
			ID: r.ID, EventID: r.EventID, Name: r.Name, OwnerID: r.OwnerID,
			OwnerUsername: r.OwnerUsername, IsLookingForMembers: r.IsLookingForMembers,
			MemberCount: r.MemberCount, CreatedAt: r.CreatedAt,
			MyPendingInvite: invitedTeams[r.ID], MyPendingRequest: requestedTeams[r.ID],
		}
		if myTeam != nil {
			v.IsMember = r.ID == *myTeam
			v.IsOwner = v.IsMember && r.OwnerID == *viewerID
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, viewerID *uuid.UUID, teamID uuid.UUID) (*TeamView, error) {
	team, err := s.store.GetTeamByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	ev, err := s.store.GetEventByID(ctx, team.EventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	members, err := s.store.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	v := TeamView{
		ID: team.ID, EventID: team.EventID, EventTitle: ev.Title, EventSlug: ev.Slug,
		EventStatus: ev.Status, Name: team.Name, OwnerID: team.OwnerID,
		IsLookingForMembers: team.IsLookingForMembers, MemberCount: int32(len(members)),
		MaxTeamSize: ev.MaxTeamSize, CreatedAt: team.CreatedAt,
	}
	if owner, err := s.store.GetUserByID(ctx, team.OwnerID); err == nil {
		v.OwnerUsername = owner.Username
	}
	v.Members = make([]MemberView, 0, len(members))
	for _, m := range members {
		v.Members = append(v.Members, MemberView{
			UserID: m.UserID, Username: m.Username, FullName: derefString(m.FullName),
			JoinedAt: m.JoinedAt, IsOwner: m.UserID == team.OwnerID,
		})
	}
	if viewerID != nil {
		v.IsMember = false
		for _, m := range members {
			if m.UserID == *viewerID {
				v.IsMember = true
			}
		}
		v.IsOwner = v.IsMember && team.OwnerID == *viewerID
		if v.IsMember {
			if _, err := s.store.GetPendingJoinRequestForTeamUser(ctx, db.GetPendingJoinRequestForTeamUserParams{TeamID: teamID, UserID: *viewerID}); err == nil {
				v.MyPendingRequest = true
			}
			if _, err := s.store.GetPendingTeamInviteForTeamUser(ctx, db.GetPendingTeamInviteForTeamUserParams{TeamID: teamID, InvitedUserID: *viewerID}); err == nil {
				v.MyPendingInvite = true
			}
		} else {
			if _, err := s.store.GetPendingJoinRequestForTeamUser(ctx, db.GetPendingJoinRequestForTeamUserParams{TeamID: teamID, UserID: *viewerID}); err == nil {
				v.MyPendingRequest = true
			}
			if _, err := s.store.GetPendingTeamInviteForTeamUser(ctx, db.GetPendingTeamInviteForTeamUserParams{TeamID: teamID, InvitedUserID: *viewerID}); err == nil {
				v.MyPendingInvite = true
			}
		}
		if v.IsOwner {
			invites, _ := s.store.ListTeamInvites(ctx, teamID)
			for _, inv := range invites {
				if inv.Status != "pending" {
					continue
				}
				v.PendingInvites = append(v.PendingInvites, InviteView{
					ID: inv.ID, TeamID: inv.TeamID, InvitedUserID: inv.InvitedUserID,
					InvitedBy: inv.InvitedBy, Status: inv.Status, TeamName: inv.TeamName,
					EventID: inv.EventID, EventTitle: inv.EventTitle,
					InvitedByUsername: inv.InvitedByUsername, CreatedAt: inv.CreatedAt,
				})
			}
			reqs, _ := s.store.ListJoinRequestsForTeam(ctx, teamID)
			for _, req := range reqs {
				v.PendingRequests = append(v.PendingRequests, JoinRequestView{
					ID: req.ID, TeamID: req.TeamID, UserID: req.UserID, Message: req.Message,
					Status: req.Status, Username: req.Username, FullName: derefString(req.FullName),
					CreatedAt: req.CreatedAt,
				})
			}
		}
	}
	return &v, nil
}

func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]TeamView, error) {
	rows, err := s.store.ListMyTeams(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]TeamView, 0, len(rows))
	for _, r := range rows {
		out = append(out, TeamView{
			ID: r.ID, EventID: r.EventID, EventTitle: r.EventTitle, EventSlug: r.EventSlug,
			EventStatus: r.EventStatus, Name: r.Name, OwnerID: r.OwnerID,
			OwnerUsername: r.OwnerUsername, IsLookingForMembers: r.IsLookingForMembers,
			MemberCount: r.MemberCount, CreatedAt: r.CreatedAt,
			IsMember: true, IsOwner: r.OwnerID == userID,
		})
	}
	return out, nil
}

func (s *Service) ListMyInvites(ctx context.Context, userID uuid.UUID) ([]InviteView, error) {
	rows, err := s.store.ListMyTeamInvites(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]InviteView, 0, len(rows))
	for _, r := range rows {
		out = append(out, InviteView{
			ID: r.ID, TeamID: r.TeamID, InvitedUserID: r.InvitedUserID, InvitedBy: r.InvitedBy,
			Status: r.Status, TeamName: r.TeamName, EventID: r.EventID, EventTitle: r.EventTitle,
			InvitedByUsername: r.InvitedByUsername, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) ListMyJoinRequests(ctx context.Context, userID uuid.UUID) ([]JoinRequestView, error) {
	rows, err := s.store.ListMyJoinRequests(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]JoinRequestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, JoinRequestView{
			ID: r.ID, TeamID: r.TeamID, UserID: r.UserID, Message: r.Message, Status: r.Status,
			TeamName: r.TeamName, EventID: r.EventID, EventTitle: r.EventTitle, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, userID, teamID uuid.UUID, name string, looking bool) error {
	team, err := s.teamOwnedBy(ctx, teamID, userID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 80 {
		return apperr.Invalid("name", "Team name must be 2–80 characters.")
	}
	ev, err := s.store.GetEventByID(ctx, team.EventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.JudgingStatus == "locked" || ev.JudgingStatus == "published" {
		return apperr.ErrConflict
	}
	if err := s.store.UpdateTeam(ctx, db.UpdateTeamParams{ID: teamID, Name: name, IsLookingForMembers: looking}); err != nil {
		if unique(err) {
			return apperr.Invalid("name", "A team with that name already exists.")
		}
		return err
	}
	return nil
}

func (s *Service) Invite(ctx context.Context, userID, teamID uuid.UUID, username string) (*InviteView, error) {
	team, err := s.teamOwnedBy(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}
	ev, err := s.store.GetEventByID(ctx, team.EventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := ensureTeamFormationOpen(ev); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	target, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, apperr.Invalid("username", "No participant with that username.")
	}
	if target.ID == userID {
		return nil, apperr.Invalid("username", "You cannot invite yourself.")
	}
	members, err := s.store.ListTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if int32(len(members)) >= int32(ev.MaxTeamSize) {
		return nil, apperr.Invalid("team", "This team is already full.")
	}
	for _, m := range members {
		if m.UserID == target.ID {
			return nil, apperr.Invalid("username", "That person is already on this team.")
		}
	}
	existing, _ := s.store.ListTeamMembershipInEvent(ctx, db.ListTeamMembershipInEventParams{EventID: team.EventID, UserID: target.ID})
	if len(existing) > 0 {
		return nil, apperr.Invalid("username", "That person is already in a team for this event.")
	}
	inv, err := s.store.CreateTeamInvite(ctx, db.CreateTeamInviteParams{TeamID: teamID, InvitedUserID: target.ID, InvitedBy: userID})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: target.ID, Kind: "team_invite", Title: "Team invite",
		Body: "You have been invited to join " + team.Name + " for " + ev.Title + ".",
		Link: "/events/" + team.EventID.String() + "/team",
	}); err != nil {
		s.log.Warn("team invite notification", "error", err)
	}
	return &InviteView{
		ID: inv.ID, TeamID: inv.TeamID, InvitedUserID: inv.InvitedUserID,
		InvitedBy: inv.InvitedBy, Status: inv.Status, CreatedAt: inv.CreatedAt,
	}, nil
}

func (s *Service) CancelInvite(ctx context.Context, userID, teamID, inviteID uuid.UUID) error {
	if _, err := s.teamOwnedBy(ctx, teamID, userID); err != nil {
		return err
	}
	inv, err := s.store.GetTeamInvite(ctx, inviteID)
	if err != nil || inv.TeamID != teamID {
		return apperr.ErrNotFound
	}
	if inv.Status != "pending" {
		return apperr.ErrConflict
	}
	return s.store.UpdateTeamInviteStatus(ctx, db.UpdateTeamInviteStatusParams{ID: inviteID, Status: "cancelled"})
}

func (s *Service) RespondInvite(ctx context.Context, userID, inviteID uuid.UUID, accept bool) (*TeamView, error) {
	inv, err := s.store.GetTeamInvite(ctx, inviteID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if inv.InvitedUserID != userID {
		return nil, apperr.ErrNotFound
	}
	if inv.Status != "pending" {
		return nil, apperr.ErrConflict
	}
	if !accept {
		if err := s.store.UpdateTeamInviteStatus(ctx, db.UpdateTeamInviteStatusParams{ID: inviteID, Status: "rejected"}); err != nil {
			return nil, err
		}
		return nil, nil
	}
	team, err := s.joinTeam(ctx, userID, inv.TeamID, func(q *db.Queries) error {
		return q.UpdateTeamInviteStatus(ctx, db.UpdateTeamInviteStatusParams{ID: inviteID, Status: "accepted"})
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: team.OwnerID, Kind: "team_joined", Title: "New teammate",
		Body: "Someone accepted your invite to " + team.Name + ".",
		Link: "/events/" + team.EventID.String() + "/team",
	}); err != nil {
		s.log.Warn("notify team join", "error", err)
	}
	return s.Get(ctx, &userID, inv.TeamID)
}

func (s *Service) RequestToJoin(ctx context.Context, userID, teamID uuid.UUID, message string) (*JoinRequestView, error) {
	team, err := s.store.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, team.EventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := ensureTeamFormationOpen(ev); err != nil {
		return nil, err
	}
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		return nil, apperr.Invalid("message", "Keep the message under 500 characters.")
	}
	existing, _ := s.store.ListTeamMembershipInEvent(ctx, db.ListTeamMembershipInEventParams{EventID: team.EventID, UserID: userID})
	if len(existing) > 0 {
		return nil, apperr.Invalid("team", "You are already in a team for this event.")
	}
	members, err := s.store.CountTeamMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if members >= int32(ev.MaxTeamSize) {
		return nil, apperr.Invalid("team", "This team is already full.")
	}
	req, err := s.store.CreateJoinRequest(ctx, db.CreateJoinRequestParams{TeamID: teamID, UserID: userID, Message: message})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: team.OwnerID, Kind: "join_request", Title: "Join request",
		Body: "Someone asked to join " + team.Name + ".", Link: "/events/" + team.EventID.String() + "/team",
	}); err != nil {
		s.log.Warn("notify join request", "error", err)
	}
	return &JoinRequestView{
		ID: req.ID, TeamID: req.TeamID, UserID: req.UserID, Message: req.Message,
		Status: req.Status, CreatedAt: req.CreatedAt,
	}, nil
}

func (s *Service) ListJoinRequests(ctx context.Context, userID, teamID uuid.UUID) ([]JoinRequestView, error) {
	if _, err := s.teamOwnedBy(ctx, teamID, userID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListJoinRequestsForTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]JoinRequestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, JoinRequestView{
			ID: r.ID, TeamID: r.TeamID, UserID: r.UserID, Message: r.Message, Status: r.Status,
			Username: r.Username, FullName: derefString(r.FullName), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Service) RespondJoinRequest(ctx context.Context, userID, requestID uuid.UUID, accept bool) (*TeamView, error) {
	req, err := s.store.GetJoinRequest(ctx, requestID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if req.Status != "pending" {
		return nil, apperr.ErrConflict
	}
	team, err := s.teamOwnedBy(ctx, req.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if !accept {
		if err := s.store.UpdateJoinRequestStatus(ctx, db.UpdateJoinRequestStatusParams{ID: requestID, Status: "rejected"}); err != nil {
			return nil, err
		}
		if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: req.UserID, Kind: "join_rejected", Title: "Join request declined",
			Body: "Your request to join " + team.Name + " was declined.",
			Link: "/events/" + team.EventID.String() + "/team",
		}); err != nil {
			s.log.Warn("notify join rejection", "error", err)
		}
		return nil, nil
	}
	joined, err := s.joinTeam(ctx, req.UserID, req.TeamID, func(q *db.Queries) error {
		return q.UpdateJoinRequestStatus(ctx, db.UpdateJoinRequestStatusParams{ID: requestID, Status: "accepted"})
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: req.UserID, Kind: "join_accepted", Title: "You're in!",
		Body: "You joined " + team.Name + ".", Link: "/events/" + team.EventID.String() + "/team",
	}); err != nil {
		s.log.Warn("notify join acceptance", "error", err)
	}
	return joined, nil
}

// joinTeam performs the size/one-team-per-event checks and membership insert
// while holding a row lock on the team, then runs `respond` in the same tx.
func (s *Service) joinTeam(ctx context.Context, userID, teamID uuid.UUID, respond func(*db.Queries) error) (*TeamView, error) {
	var team db.Team
	err := s.store.InTx(ctx, func(q *db.Queries) error {
		t, err := q.LockTeamForUpdate(ctx, teamID)
		if err != nil {
			return apperr.ErrNotFound
		}
		team = t
		ev, err := q.GetEventByID(ctx, t.EventID)
		if err != nil {
			return apperr.ErrNotFound
		}
		if err := ensureTeamFormationOpen(ev); err != nil {
			return err
		}
		membership, err := q.ListTeamMembershipInEvent(ctx, db.ListTeamMembershipInEventParams{EventID: t.EventID, UserID: userID})
		if err != nil {
			return err
		}
		if len(membership) > 0 {
			return apperr.Invalid("team", "You are already in a team for this event.")
		}
		count, err := q.CountTeamMembers(ctx, teamID)
		if err != nil {
			return err
		}
		if count >= int32(ev.MaxTeamSize) {
			return apperr.Invalid("team", "This team is full.")
		}
		if err := q.AddTeamMember(ctx, db.AddTeamMemberParams{EventID: t.EventID, TeamID: teamID, UserID: userID}); err != nil {
			if unique(err) {
				return apperr.Invalid("team", "You are already in a team for this event.")
			}
			return err
		}
		// A registered team automatically registers late joiners with the same answers.
		if reg, err := q.GetTeamRegistration(ctx, db.GetTeamRegistrationParams{EventID: t.EventID, TeamID: &teamID}); err == nil {
			if _, err := q.CreateRegistrationForTeamMember(ctx, db.CreateRegistrationForTeamMemberParams{
				EventID: t.EventID, UserID: userID, TeamID: &teamID, FormAnswers: reg.FormAnswers,
			}); err != nil {
				return err
			}
		}
		if err := q.CancelTeamInvitesForEventUser(ctx, db.CancelTeamInvitesForEventUserParams{InvitedUserID: userID, EventID: t.EventID}); err != nil {
			return err
		}
		if err := q.CancelJoinRequestsForEventUser(ctx, db.CancelJoinRequestsForEventUserParams{UserID: userID, EventID: t.EventID}); err != nil {
			return err
		}
		return respond(q)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, &userID, team.ID)
}

// Leave removes the caller from a team. Owners transfer ownership to the
// longest-standing remaining member, or the team dissolves when empty.
func (s *Service) Leave(ctx context.Context, userID, teamID uuid.UUID) error {
	var becameOwner *uuid.UUID
	err := s.store.InTx(ctx, func(q *db.Queries) error {
		team, err := q.LockTeamForUpdate(ctx, teamID)
		if err != nil {
			return apperr.ErrNotFound
		}
		if _, err := q.GetTeamMember(ctx, db.GetTeamMemberParams{TeamID: teamID, UserID: userID}); err != nil {
			return apperr.ErrNotFound
		}
		members, err := q.ListTeamMembers(ctx, teamID)
		if err != nil {
			return err
		}
		if err := q.DeleteRegistrationForTeamUser(ctx, db.DeleteRegistrationForTeamUserParams{EventID: team.EventID, UserID: userID}); err != nil {
			return err
		}
		if err := q.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{TeamID: teamID, UserID: userID}); err != nil {
			return err
		}
		if err := q.CancelTeamInvitesForEventUser(ctx, db.CancelTeamInvitesForEventUserParams{InvitedUserID: userID, EventID: team.EventID}); err != nil {
			return err
		}
		if err := q.CancelJoinRequestsForEventUser(ctx, db.CancelJoinRequestsForEventUserParams{UserID: userID, EventID: team.EventID}); err != nil {
			return err
		}
		if team.OwnerID != userID {
			return nil
		}
		var next *uuid.UUID
		for _, m := range members {
			if m.UserID == userID {
				continue
			}
			uid := m.UserID
			next = &uid
			break
		}
		if next == nil {
			if err := q.DeleteRegistrationsForTeam(ctx, &teamID); err != nil {
				return err
			}
			return q.DeleteTeam(ctx, teamID)
		}
		if err := q.UpdateTeamOwner(ctx, db.UpdateTeamOwnerParams{ID: teamID, OwnerID: *next}); err != nil {
			return err
		}
		becameOwner = next
		return nil
	})
	if err != nil {
		return err
	}
	if becameOwner != nil {
		if _, err := s.store.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: *becameOwner, Kind: "team_owner", Title: "You're now the team owner",
			Body: "Ownership was transferred because the previous owner left.", Link: "/dashboard",
		}); err != nil {
			s.log.Warn("notify ownership transfer", "error", err)
		}
	}
	return nil
}

func (s *Service) Dissolve(ctx context.Context, userID, teamID uuid.UUID) error {
	team, err := s.teamOwnedBy(ctx, teamID, userID)
	if err != nil {
		return err
	}
	members, err := s.store.ListTeamMembers(ctx, teamID)
	if err != nil {
		return err
	}
	if len(members) > 1 {
		return apperr.Invalid("team", "Other members must leave before you can dissolve this team.")
	}
	if _, err := s.store.GetSubmissionByTeam(ctx, db.GetSubmissionByTeamParams{EventID: team.EventID, TeamID: &teamID}); err == nil {
		return apperr.Invalid("team", "This team has a submission and cannot be dissolved.")
	}
	return s.store.InTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteRegistrationsForTeam(ctx, &teamID); err != nil {
			return err
		}
		return q.DeleteTeam(ctx, teamID)
	})
}

func (s *Service) teamOwnedBy(ctx context.Context, teamID, userID uuid.UUID) (db.Team, error) {
	team, err := s.store.GetTeamByID(ctx, teamID)
	if err != nil {
		return db.Team{}, apperr.ErrNotFound
	}
	if team.OwnerID != userID {
		return db.Team{}, apperr.ErrNotFound
	}
	return team, nil
}

func ensureTeamFormationOpen(ev db.GetEventByIDRow) error {
	now := time.Now()
	if ev.Status != "published" {
		return apperr.Invalid("event", "This event is not open for team formation.")
	}
	if ev.RegistrationOpensAt != nil && ev.RegistrationOpensAt.After(now) {
		return apperr.Invalid("event", "Registration has not opened yet.")
	}
	if ev.RegistrationClosesAt != nil && ev.RegistrationClosesAt.Before(now) {
		return apperr.Invalid("event", "Registration has closed.")
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func unique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Service) ListInvites(ctx context.Context, userID, teamID uuid.UUID) ([]InviteView, error) {
	if _, err := s.teamOwnedBy(ctx, teamID, userID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListTeamInvites(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]InviteView, 0, len(rows))
	for _, r := range rows {
		out = append(out, InviteView{
			ID: r.ID, TeamID: r.TeamID, InvitedUserID: r.InvitedUserID, InvitedBy: r.InvitedBy,
			Status: r.Status, TeamName: r.TeamName, EventID: r.EventID, EventTitle: r.EventTitle,
			InvitedByUsername: r.InvitedByUsername, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// GetForEventUser returns nil when the user has no team for the event.
func (s *Service) GetForEventUser(ctx context.Context, userID, eventID uuid.UUID) (*TeamView, error) {
	team, err := s.store.GetTeamForEventUser(ctx, db.GetTeamForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s.Get(ctx, &userID, team.ID)
}
