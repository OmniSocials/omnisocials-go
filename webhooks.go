package omnisocials

import (
	"context"
	"encoding/json"
	"net/url"
)

// WebhooksService covers the /webhooks management endpoints. For verifying
// incoming deliveries, see VerifyWebhookSignature.
type WebhooksService struct {
	client *Client
}

// WebhookFailure describes a webhook endpoint's most recent delivery failure.
type WebhookFailure struct {
	At       string `json:"at"`
	Status   *int   `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	Attempts int    `json:"attempts,omitempty"`
}

// XStatisticsWebhookEvents are the X statistics webhook events:
// x_statistics.paused (the organisation's X statistics stopped at 0
// credits) and x_statistics.resumed (they started again after a top-up).
// One delivery per workspace with X; the data is XStatisticsEventData.
var XStatisticsWebhookEvents = []string{
	"x_statistics.paused",
	"x_statistics.resumed",
}

// XStatisticsEventData is the `data` of an x_statistics.paused or
// x_statistics.resumed delivery. Decode the raw delivery into a struct with
// `Data XStatisticsEventData` after VerifyWebhookSignature accepted it.
type XStatisticsEventData struct {
	// WorkspaceID arrives as a number or a string.
	WorkspaceID json.Number `json:"workspace_id"`
	// AccountID is the X account as in Accounts.List ("<workspace_id>_x").
	AccountID string `json:"account_id"`
	// Reason is "insufficient_credits" (paused) or "credits_added"
	// (resumed).
	Reason string `json:"reason"`
}

// Webhook is a registered webhook endpoint.
type Webhook struct {
	ID  string `json:"id"`
	URL string `json:"url"`
	// Events the endpoint is subscribed to: post.scheduled, post.published,
	// post.failed, post.approved, post.rejected, the ads events
	// (AdsWebhookEvents) and the X statistics events
	// (XStatisticsWebhookEvents).
	Events []string `json:"events"`
	// WorkspaceID is the workspace whose events the webhook receives. nil
	// for an organisation-wide webhook (made in the OmniSocials dashboard),
	// which receives the events of every workspace of the organisation.
	// Webhooks created through the API belong to the key's workspace.
	WorkspaceID *int64 `json:"workspace_id"`
	// IsActive is false when the webhook is switched off: by an Update, or
	// automatically when the person behind it lost access to its
	// organisation or workspace. An automatic switch-off says why in
	// LastFailure.Error (Status nil, Attempts 0).
	// Update with IsActive true switches it on again.
	IsActive        bool            `json:"is_active"`
	LastTriggeredAt *string         `json:"last_triggered_at,omitempty"`
	FailureCount    int             `json:"failure_count,omitempty"`
	LastFailure     *WebhookFailure `json:"last_failure,omitempty"`
	CreatedAt       string          `json:"created_at,omitempty"`
	// Secret is the signing secret; only returned on Create and
	// RotateSecret. Save it, it is only shown once.
	Secret string `json:"secret,omitempty"`
}

// WebhookCreateParams is the request body for Webhooks.Create.
type WebhookCreateParams struct {
	// URL is the HTTPS endpoint that will receive event deliveries.
	URL string `json:"url"`
	// Events to subscribe to: post.scheduled, post.published, post.failed,
	// post.approved, post.rejected, and the ads events (AdsWebhookEvents):
	// ad.status_changed (its data is the campaign, see
	// AdStatusChangedEventData), ad.created, ad.updated, ad.review,
	// ad.delivery_issue, campaign.created, campaign.updated,
	// campaign.budget, lead.received, ads.sync_failed, ads.sync_recovered,
	// ads.initial_sync_completed, ad_rule.triggered, and the X statistics
	// events (XStatisticsWebhookEvents): x_statistics.paused,
	// x_statistics.resumed (their data is XStatisticsEventData). Any ads
	// event needs the ads:read scope on the key and access_ads for its
	// person (else 403 insufficient_scope / insufficient_permission).
	Events []string `json:"events"`
}

// WebhookUpdateParams is the request body for Webhooks.Update.
type WebhookUpdateParams struct {
	URL      string   `json:"url,omitempty"`
	Events   []string `json:"events,omitempty"`
	IsActive *bool    `json:"is_active,omitempty"`
}

// RotatedWebhookSecret is the payload of Webhooks.RotateSecret.
type RotatedWebhookSecret struct {
	ID string `json:"id"`
	// Secret is the new signing secret. Save it, it is only shown once.
	Secret string `json:"secret"`
}

// RotateWebhookSecretResponse is the Webhooks.RotateSecret response.
type RotateWebhookSecretResponse struct {
	Data    RotatedWebhookSecret `json:"data"`
	Message string               `json:"message,omitempty"`
}

// List calls `GET /webhooks`: the webhooks of the key's workspace and the
// organisation-wide webhooks of its organisation (WorkspaceID nil, made in
// the OmniSocials dashboard; they receive the events of every workspace). A
// webhook whose creator lost access to the organisation or workspace is
// switched off automatically: IsActive is false and LastFailure.Error says
// why.
func (s *WebhooksService) List(ctx context.Context) (*ListResponse[Webhook], error) {
	var out ListResponse[Webhook]
	if err := s.client.get(ctx, "/webhooks", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /webhooks/:id`: fetch a webhook of the key's workspace, or
// an organisation-wide webhook of its organisation (WorkspaceID nil).
func (s *WebhooksService) Get(ctx context.Context, id string) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.get(ctx, "/webhooks/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /webhooks`: register an endpoint for event deliveries
// (post.scheduled, post.published, post.failed, post.approved,
// post.rejected, the ads events in AdsWebhookEvents and the X statistics
// events in XStatisticsWebhookEvents). An ads event needs
// the ads:read scope on the key and access_ads for its person (else 403
// insufficient_scope / insufficient_permission). The response includes the
// signing Secret; save it, it is only shown once.
func (s *WebhooksService) Create(ctx context.Context, params *WebhookCreateParams) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.post(ctx, "/webhooks", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /webhooks/:id`: change the URL, events, or active
// state. Works on organisation-wide webhooks too. IsActive true switches on
// a webhook that was switched off (also an automatic switch-off); setting
// URL, Events or switching it on makes the key's creator the person behind
// the webhook. Setting URL on an organisation-wide webhook (even its current
// one) moves it to the key's workspace: from then on it gets only that
// workspace's events, and the answer shows the new WorkspaceID. When the
// webhook has Ads events after the change, setting URL or Events or
// switching it on needs the ads:read scope and access_ads for the key's
// creator (403 otherwise, nothing changed).
func (s *WebhooksService) Update(ctx context.Context, id string, params *WebhookUpdateParams) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.patch(ctx, "/webhooks/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /webhooks/:id`: delete a webhook of the key's
// workspace or an organisation-wide webhook of its organisation (204 on
// success).
func (s *WebhooksService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/webhooks/"+url.PathEscape(id))
}

// RotateSecret calls `POST /webhooks/:id/rotate-secret`: generate a new
// signing secret. Save the returned secret; it is only shown once. The old
// secret keeps signing for 24 hours: every delivery then carries
// X-OmniSocials-Signature (new secret) and X-OmniSocials-Signature-Previous
// (old secret, same format, same timestamp and body). Verify the first with
// the new secret and, when that fails, the second with the old one (see
// VerifyWebhookSignature). Rotating again within the 24 hours replaces the
// previous secret with the one just rotated out. Works on organisation-wide
// webhooks too.
func (s *WebhooksService) RotateSecret(ctx context.Context, id string) (*RotateWebhookSecretResponse, error) {
	var out RotateWebhookSecretResponse
	if err := s.client.post(ctx, "/webhooks/"+url.PathEscape(id)+"/rotate-secret", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
