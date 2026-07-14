package omnisocials

import (
	"context"
	"net/url"
)

// AccountsService covers the /accounts endpoints.
type AccountsService struct {
	client *Client
}

// AccountBoard is a Pinterest board on a connected Pinterest account.
type AccountBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Account is a connected social account.
type Account struct {
	ID             string  `json:"id"`
	Platform       string  `json:"platform"`
	Username       string  `json:"username"`
	DisplayName    string  `json:"display_name"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
	// ContentTypes lists what the account can publish, e.g. ["post",
	// "story", "reel"].
	ContentTypes []string `json:"content_types,omitempty"`
	// Status is "active" while the OAuth token is healthy. It flips to
	// "needs_reconnect" when the platform has revoked/expired the token;
	// posts to this account will fail until the user reconnects.
	Status         string `json:"status"`
	NeedsReconnect bool   `json:"needs_reconnect"`
	// ReauthReason is a short reason from the platform; only present when
	// NeedsReconnect is true.
	ReauthReason *string `json:"reauth_reason,omitempty"`
	ConnectedAt  *string `json:"connected_at,omitempty"`
	// Boards is only present for Pinterest accounts.
	Boards          []AccountBoard `json:"boards,omitempty"`
	PlatformDetails map[string]any `json:"platform_details,omitempty"`
}

// List calls `GET /accounts`: the workspace's connected social accounts.
func (s *AccountsService) List(ctx context.Context) (*ListResponse[Account], error) {
	var out ListResponse[Account]
	if err := s.client.get(ctx, "/accounts", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /accounts/:id`: a single connected account.
func (s *AccountsService) Get(ctx context.Context, id string) (*ItemResponse[Account], error) {
	var out ItemResponse[Account]
	if err := s.client.get(ctx, "/accounts/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
