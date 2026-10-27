export interface User {
  id: string;
  email: string;
  username: string;
  onboarding_step: number;
  onboarding_completed: boolean;
  intents: string[];
  is_admin: boolean;
}

export interface Profile {
  full_name: string;
  college: string;
  course: string;
  year_of_study: number | null;
  updated_at?: string;
}

export interface MeResponse {
  user: User | null;
  profile: Profile | null;
  google_enabled: boolean;
}

export interface Criterion {
  name: string;
  description: string;
}

export interface JudgingConfig {
  target_comparisons: number;
  judge_budget: number;
  anchor_count: number;
}

export type EventStatus = "draft" | "published" | "closed" | "results_published";

export interface EventView {
  id: string;
  slug: string;
  organizer_id: string;
  organizer_username: string;
  title: string;
  description: string;
  category: string;
  participation_format: "individual" | "team" | "both";
  status: EventStatus;
  eligibility: string;
  rules: string;
  contact_email: string;
  location: string;
  min_team_size: number;
  max_team_size: number;
  registration_opens_at: string | null;
  registration_closes_at: string | null;
  starts_at: string | null;
  ends_at: string | null;
  submission_deadline: string | null;
  criteria: Criterion[];
  judging_status: "setup" | "live" | "locked" | "published";
  judging_config: JudgingConfig;
  results_published_at: string | null;
  registration_status: string;
  registration_count: number;
  submission_count: number;
  can_manage: boolean;
  is_judge: boolean;
  is_registered: boolean;
  created_at: string;
  updated_at: string;
  form_fields?: FormField[];
}

export interface FormField {
  id: string;
  label: string;
  field_type: "text" | "long_text" | "number" | "url" | "select" | "multi_select" | "file" | "checkbox";
  required: boolean;
  options: string[];
  help_text: string;
  position: number;
}

export interface Registration {
  id: string;
  event_id: string;
  event_title?: string;
  event_slug?: string;
  event_status?: string;
  starts_at?: string | null;
  user_id: string;
  username?: string;
  full_name?: string;
  college?: string;
  email?: string;
  team_id: string | null;
  team_name?: string;
  answers: Record<string, unknown>;
  status: "registered" | "waitlisted" | "cancelled" | "disqualified";
  has_submission: boolean;
  created_at: string;
  updated_at: string;
}

export interface Submission {
  id: string;
  event_id: string;
  event_title?: string;
  event_slug?: string;
  submission_deadline?: string | null;
  registration_id: string | null;
  team_id: string | null;
  team_name?: string;
  created_by: string;
  creator_username?: string;
  creator_name?: string;
  title: string;
  description: string;
  repo_url: string;
  demo_url: string;
  file_url: string;
  status: "draft" | "submitted";
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  user_id: string;
  username: string;
  full_name: string;
  joined_at: string;
  is_owner: boolean;
}

export interface TeamInvite {
  id: string;
  team_id: string;
  invited_user_id: string;
  invited_by: string;
  status: string;
  team_name?: string;
  event_id?: string;
  event_title?: string;
  invited_by_username?: string;
  created_at: string;
}

export interface JoinRequest {
  id: string;
  team_id: string;
  user_id: string;
  message: string;
  status: string;
  username?: string;
  full_name?: string;
  team_name?: string;
  event_id?: string;
  event_title?: string;
  created_at: string;
}

export interface Team {
  id: string;
  event_id: string;
  event_title?: string;
  event_slug?: string;
  event_status?: string;
  name: string;
  owner_id: string;
  owner_username: string;
  is_looking_for_members: boolean;
  member_count: number;
  max_team_size?: number;
  created_at: string;
  members?: TeamMember[];
  is_member: boolean;
  is_owner: boolean;
  my_pending_invite: boolean;
  my_pending_request: boolean;
  pending_invites?: TeamInvite[];
  pending_requests?: JoinRequest[];
}

export interface AssignmentPair {
  judge_id: string;
  submission_a: string;
  submission_b: string;
  left_submission: string;
  anchor_pair: boolean;
}

export interface AssignmentPlan {
  pairs: AssignmentPair[];
  judge_stats: Array<{ judge_id: string; comparisons: number; submissions_seen: number }>;
  overlap: Array<{ judge_a: string; judge_b: string; shared: number }>;
  coverage: Record<string, number>;
  coverage_min: number;
  coverage_max: number;
  connected: boolean;
  anchors: string[];
  total_pairs: number;
  target_coverage: number;
}

export interface JudgingPreview {
  config: JudgingConfig;
  submissions: JudgingSubmission[];
  judges: Array<{ id: string; username: string; email: string; load?: number }>;
  plan: AssignmentPlan;
  warnings: string[];
}

export interface JudgingSubmission {
  id: string;
  title: string;
  description: string;
  repo_url: string;
  demo_url: string;
  file_url: string;
  team_id: string | null;
  team_name?: string;
  creator_username: string;
  creator_name?: string;
  created_by: string;
}

export interface JudgePair {
  id: string;
  position: number;
  status: string;
  left_submission: string;
  a: JudgingSubmission;
  b: JudgingSubmission;
}

export interface JudgingNext {
  pair: JudgePair | null;
  progress: { total: number; done: number; last_pair_id?: string | null };
  criteria: Criterion[];
}

export interface RankingEntry {
  submission_id: string;
  title: string;
  team_name?: string;
  creator_username: string;
  score: number;
  variance: number;
  confidence: number;
  rank: number;
  coverage: number;
  wins: number;
  losses: number;
}

export interface JudgeReliability {
  id: string;
  username?: string;
  email?: string;
  reliability: number;
  alpha: number;
  beta: number;
  comparisons: number;
  assigned: number;
  done: number;
}

export interface Disagreement {
  pair_id: string;
  title_a: string;
  title_b: string;
  judgments: number;
  winner_counts: Record<string, number>;
}

export interface JudgingReview {
  judging_status: string;
  total_pairs: number;
  done_pairs: number;
  pending_pairs: number;
  total_judgments: number;
  coverage_min: number;
  coverage_max: number;
  criteria: Criterion[];
  rankings: RankingEntry[];
  judges: JudgeReliability[];
  disagreements: Disagreement[];
  warnings: string[];
}

export interface ResultEntry {
  rank: number;
  submission_id: string;
  title: string;
  team_name?: string;
  creator_username: string;
  score: number;
  confidence: number;
}

export interface Notification {
  id: string;
  kind: string;
  title: string;
  body: string;
  link: string;
  read_at: string | null;
  created_at: string;
}

export interface Bookmark {
  event_id: string;
  slug: string;
  title: string;
  status: string;
  starts_at?: string | null;
  bookmarked_at: string;
}

export interface ChatMessage {
  id: string;
  team_id: string;
  user_id: string;
  body: string;
  username: string;
  full_name?: string;
  created_at: string;
}

export interface DashboardData {
  registrations: Registration[];
  teams: Team[];
  team_invites: TeamInvite[];
  join_requests: JoinRequest[];
  submissions: Submission[];
  hosted_events: EventView[];
  judge_events: EventView[];
  unread_notifications: number;
}

export interface StaffMember {
  id: string;
  event_id: string;
  user_id: string | null;
  email: string;
  role: string;
  status: string;
  invited_by: string;
  invite_token?: string | null;
  accept_url?: string;
  username?: string | null;
  created_at: string;
}
