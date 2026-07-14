package omnisocials

import (
	"context"
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

// Webhook is a registered webhook endpoint.
type Webhook struct {
	ID  string `json:"id"`
	URL string `json:"url"`
	// Events the endpoint is subscribed to: post.scheduled, post.published,
	// post.failed.
	Events          []string        `json:"events"`
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
	// Events to subscribe to: post.scheduled, post.published, post.failed.
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

// List calls `GET /webhooks`: the workspace's webhook endpoints.
func (s *WebhooksService) List(ctx context.Context) (*ListResponse[Webhook], error) {
	var out ListResponse[Webhook]
	if err := s.client.get(ctx, "/webhooks", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /webhooks/:id`: fetch a single webhook.
func (s *WebhooksService) Get(ctx context.Context, id string) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.get(ctx, "/webhooks/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /webhooks`: register an endpoint for event deliveries
// (post.scheduled, post.published, post.failed). The response includes the
// signing Secret; save it, it is only shown once.
func (s *WebhooksService) Create(ctx context.Context, params *WebhookCreateParams) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.post(ctx, "/webhooks", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /webhooks/:id`: change the URL, events, or active
// state.
func (s *WebhooksService) Update(ctx context.Context, id string, params *WebhookUpdateParams) (*ItemResponse[Webhook], error) {
	var out ItemResponse[Webhook]
	if err := s.client.patch(ctx, "/webhooks/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /webhooks/:id`: delete a webhook (204 on success).
func (s *WebhooksService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/webhooks/"+url.PathEscape(id))
}

// RotateSecret calls `POST /webhooks/:id/rotate-secret`: generate a new
// signing secret. Save the returned secret; it is only shown once and the old
// secret stops working immediately.
func (s *WebhooksService) RotateSecret(ctx context.Context, id string) (*RotateWebhookSecretResponse, error) {
	var out RotateWebhookSecretResponse
	if err := s.client.post(ctx, "/webhooks/"+url.PathEscape(id)+"/rotate-secret", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
