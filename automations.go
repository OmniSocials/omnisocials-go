package omnisocials

import (
	"context"
	"net/url"
	"strconv"
)

// AutomationsService covers the /automations endpoints: comment-to-DM
// automations. An automation answers a keyword in an Instagram comment (or a
// DM, or a story reply) with a DM, once per person, with an optional public
// reply under the comment. On a Facebook Page it answers a keyword in a
// comment (or a Messenger message to the Page) in Messenger; a Page has no
// story replies. On X it answers a keyword in a DM to the X account only (no
// post, no public reply), while X DMs are on for the workspace; every X
// answer ends with "Reply STOP to stop automatic messages." and a person who
// replies STOP gets no automatic DM from the account again. Meta rules: one
// private reply per comment, within 7 days. OmniSocials caps sends at 180
// DMs an hour per account on Instagram and Facebook, and at 15 per 15
// minutes and 450 a day on X; an answer over a cap waits in a queue
// ("queued") for up to an hour. The scopes are opt-in on the API key: automations:read for List, Get and
// ListEvents, automations:write for Create, Update and Delete (and for
// PostCreateParams.CommentDMAutomation).
type AutomationsService struct {
	client *Client
}

// List calls `GET /automations`: the workspace's comment-to-DM automations,
// optionally filtered by Status ("active", "paused", "completed"), by
// Platform ("instagram", "facebook", "x") or by the OmniSocials PostID they are
// bound to. A nil params value lists them all. Requires the
// automations:read scope.
func (s *AutomationsService) List(ctx context.Context, params *AutomationListParams) (*ListResponse[Automation], error) {
	query := url.Values{}
	if params != nil {
		if params.Status != "" {
			query.Set("status", params.Status)
		}
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.PostID != "" {
			query.Set("post_id", params.PostID)
		}
	}
	var out ListResponse[Automation]
	if err := s.client.get(ctx, "/automations", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /automations/:id`: fetch a single automation. Requires the
// automations:read scope.
func (s *AutomationsService) Get(ctx context.Context, id string) (*ItemResponse[Automation], error) {
	var out ItemResponse[Automation]
	if err := s.client.get(ctx, "/automations/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /automations`: create a comment-to-DM automation.
// Platform "facebook" runs it on a Facebook Page (the DM lands in
// Messenger); Platform "x" answers DMs to the X account (Trigger.Kinds
// ["dm"], no post; an active one needs X DMs on, else 409 x_dms_off, while
// Status "paused" always saves). Bind it to one post with PostID (an OmniSocials post that goes
// to the channel, scheduled or live; one automation per post and platform),
// InstagramMediaID (an Instagram post published outside OmniSocials) or
// FacebookPostID (a Page post published outside OmniSocials; implies
// "facebook"), at most one; a Trigger with Scope "next_post" or "any" needs
// none. Actions.DM.Text is required. Requires the automations:write scope.
// Errors: 400 validation_error, 400 post_not_instagram, 400
// post_not_facebook, 400 post_required, 404 post_not_found, 404
// instagram_media_not_found, 404 facebook_post_not_found, 409
// instagram_not_connected, 409 instagram_needs_reauth, 409
// facebook_not_connected, 409 facebook_needs_reauth, 409 x_not_connected,
// 409 x_dms_off, 409 automation_exists (the error body also carries
// automation_id).
func (s *AutomationsService) Create(ctx context.Context, params *AutomationCreateParams) (*ItemResponse[Automation], error) {
	var out ItemResponse[Automation]
	if err := s.client.post(ctx, "/automations", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /automations/:id`: partial update. Each section
// (Trigger, Actions, Limits) is merged into the stored one, fields you leave
// out keep their value: new Trigger.Keywords keep the kinds, the scope and
// the match; a new Actions.DM.Text keeps the public reply; Limits with only
// StopAt keeps MaxSends. PublicReply.Enabled omnisocials.Bool(false) turns
// the public reply off; a Limits map with "max_sends": omnisocials.Null
// removes that limit (see AutomationUpdateParams.Limits). Status "paused"
// pauses and "active" resumes; another PostID / InstagramMediaID /
// FacebookPostID moves the automation to that post. The platform cannot
// change (400 platform_immutable). Status "active" on an X automation needs
// X DMs on for the workspace (409 x_dms_off). A "completed" automation
// starts again when its limits or trigger change. Requires the
// automations:write scope.
// 400 no_fields when the body is empty.
func (s *AutomationsService) Update(ctx context.Context, id string, params *AutomationUpdateParams) (*ItemResponse[Automation], error) {
	var out ItemResponse[Automation]
	if err := s.client.patch(ctx, "/automations/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /automations/:id`: delete an automation (204 on
// success). Requires the automations:write scope.
func (s *AutomationsService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/automations/"+url.PathEscape(id))
}

// ListEvents calls `GET /automations/:id/events`: the automation's activity,
// one event per person it answered, skipped or failed on. Filter by Outcome
// ("sent", "skipped", "failed", "pending", "queued", "opted_out"); Limit is
// 1 to 100 (default 25),
// Offset skips. A nil params value uses the defaults. Requires the
// automations:read scope.
//
// Note the response shape (see AutomationEventsResponse): not the plain list
// envelope but Data, Counts (the number of events per outcome) and
// Pagination.
func (s *AutomationsService) ListEvents(ctx context.Context, id string, params *AutomationEventListParams) (*AutomationEventsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Outcome != "" {
			query.Set("outcome", params.Outcome)
		}
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset > 0 {
			query.Set("offset", strconv.Itoa(params.Offset))
		}
	}
	var out AutomationEventsResponse
	if err := s.client.get(ctx, "/automations/"+url.PathEscape(id)+"/events", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
