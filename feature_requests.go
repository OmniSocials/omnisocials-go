package omnisocials

import (
	"context"
	"net/url"
	"strconv"
)

// FeatureRequestsService covers /feature-requests: requests to the
// OmniSocials team, for the person who owns the key (their name and email
// are stored with it so the team can answer). The person sees them in
// OmniSocials (Settings, Request a feature) and gets an email when the
// status changes. No scope needed.
//
// Error codes: 400 invalid_summary, invalid_title, invalid_platform,
// invalid_area, unknown_limit, invalid_id; 404 not_found; 409
// not_requestable (the LimitID is a platform rule or a choice of
// OmniSocials: nothing was stored, the message gives the reason and what to
// do instead, and the entry is in the error body under error.limit); 429
// too_many_requests (10 an hour per person).
type FeatureRequestsService struct {
	client *Client
}

// The values of FeatureRequestCreateResult.Outcome.
const (
	// FeatureRequestCreated means a new request was stored.
	FeatureRequestCreated = "created"
	// FeatureRequestVoted means a vote was added to the organisation's open
	// request for the same gap.
	FeatureRequestVoted = "voted"
	// FeatureRequestAlreadyRequested means this person already has an open
	// request for it; nothing new was stored.
	FeatureRequestAlreadyRequested = "already_requested"
)

// FeatureRequestTeamReply is a reply of the OmniSocials team on a request.
type FeatureRequestTeamReply struct {
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// FeatureRequest is a feature request of the person who owns the key.
type FeatureRequest struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// Status is "open", "in_progress", "completed" or "declined".
	Status string `json:"status"`
	// TeamAnswer is the OmniSocials team's answer, when there is one.
	TeamAnswer *string `json:"team_answer"`
	// TeamReplies are the replies of the OmniSocials team, oldest first.
	TeamReplies []FeatureRequestTeamReply `json:"team_replies"`
	// VotesCount is the number of people of the organisation who asked for
	// the same thing.
	VotesCount int     `json:"votes_count"`
	LimitID    *string `json:"limit_id"`
	Platform   *string `json:"platform"`
	Area       *string `json:"area"`
	// Source is where the request was sent from: "app", "mcp", "api" or
	// "cli".
	Source      string  `json:"source"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	CompletedAt *string `json:"completed_at"`
}

// FeatureRequestCreateParams is the request body for FeatureRequests.Create.
type FeatureRequestCreateParams struct {
	// Summary is what the person wants to do and why, in one or two
	// sentences (10 to 1000 characters). Write it yourself; never paste a
	// conversation.
	Summary string `json:"summary"`
	// Title is a short name, at most 120 characters. Made from the summary
	// when left empty.
	Title string `json:"title,omitempty"`
	// Platform is the platform it is about, when it is about one (same
	// values as LimitListParams.Platform).
	Platform string `json:"platform,omitempty"`
	// Area is one of LimitAreas.
	Area string `json:"area,omitempty"`
	// LimitID is the ID of the Limits.List entry the request is about. A
	// second person of the same organisation then adds a vote to the open
	// request instead of a new one.
	LimitID string `json:"limit_id,omitempty"`
}

// FeatureRequestCreateResult is the response of FeatureRequests.Create.
type FeatureRequestCreateResult struct {
	// Outcome is FeatureRequestCreated, FeatureRequestVoted or
	// FeatureRequestAlreadyRequested.
	Outcome string `json:"outcome"`
	// Data is the request the outcome is about.
	Data FeatureRequest `json:"data"`
}

// Create calls `POST /feature-requests`: send a request. Send it only when
// the person asked for it, and check Outcome: only FeatureRequestCreated is
// a new request.
func (s *FeatureRequestsService) Create(ctx context.Context, params *FeatureRequestCreateParams) (*FeatureRequestCreateResult, error) {
	var out FeatureRequestCreateResult
	if err := s.client.post(ctx, "/feature-requests", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List calls `GET /feature-requests`: the person's own requests, newest
// first, at most 50.
func (s *FeatureRequestsService) List(ctx context.Context) (*ListResponse[FeatureRequest], error) {
	var out ListResponse[FeatureRequest]
	if err := s.client.get(ctx, "/feature-requests", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /feature-requests/:id`: one request with its status and
// the team's replies.
func (s *FeatureRequestsService) Get(ctx context.Context, id int64) (*ItemResponse[FeatureRequest], error) {
	var out ItemResponse[FeatureRequest]
	path := "/feature-requests/" + url.PathEscape(strconv.FormatInt(id, 10))
	if err := s.client.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
