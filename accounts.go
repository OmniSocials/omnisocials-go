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
	// "needs_reconnect" when the platform has revoked/expired the token
	// (Bluesky: the sign-in could not be renewed); posts to this account
	// will fail until the user reconnects. "pick_pending" (Facebook,
	// OmniSocials Connect customers only): the Facebook login manages
	// several Pages and none is chosen yet; posts fail until the Page is
	// chosen by connecting Facebook again and picking the Page.
	Status         string `json:"status"`
	NeedsReconnect bool   `json:"needs_reconnect"`
	// PickPending is true when Status is "pick_pending" (Facebook,
	// OmniSocials Connect customers only). Such an account cannot be posted
	// to, and Username, DisplayName and
	// ProfilePicture name the person who signed in, not a Page.
	PickPending bool `json:"pick_pending"`
	// ReauthReason is a short reason from the platform; only present when
	// NeedsReconnect is true.
	ReauthReason *string `json:"reauth_reason,omitempty"`
	ConnectedAt  *string `json:"connected_at,omitempty"`
	// Boards is only present for Pinterest accounts.
	Boards []AccountBoard `json:"boards,omitempty"`
	// PlatformDetails holds platform specific fields. Telegram: chat_type,
	// chat_id, subscribers, url. Discord: guild_id, guild_name (nil for a
	// pasted webhook URL), channel_id, channel_name (nil when unknown),
	// channel_type ("text", "voice", "announcement", "stage", "forum",
	// "media" or nil), webhook_name, connect_method ("oauth" or "url") and
	// url (discord.com/channels/<guild>/<channel>).
	PlatformDetails map[string]any `json:"platform_details,omitempty"`
	// XStatistics is set on X accounts only: how often OmniSocials reads
	// the account's X statistics, who pays and whether the reads are
	// paused. X bills every read; OmniSocials passes X's price on as
	// credits (half a credit per post check, plus half a credit for a video
	// or GIF, 1 credit per follower count). nil on other platforms and when
	// the setting could not be read.
	XStatistics *AccountXStatistics `json:"x_statistics,omitempty"`
}

// Values of AccountXStatistics.Posts and XStatisticsInput.Posts.
const (
	// XStatisticsPostsFull checks each post 5 times: about 5 hours, day 1,
	// 3 and 7, and day 30 for posts with 100+ views.
	XStatisticsPostsFull = "full"
	// XStatisticsPostsLight checks each post 2 times: about 5 hours and
	// day 7.
	XStatisticsPostsLight = "light"
	// XStatisticsPostsCustom follows PostSchedule (OmniSocials Connect keys
	// only).
	XStatisticsPostsCustom = "custom"
	// XStatisticsOff switches post checks or follower counts off.
	XStatisticsOff = "off"
)

// Values of AccountXStatistics.Followers and XStatisticsInput.Followers.
const (
	XStatisticsFollowersDaily  = "daily"
	XStatisticsFollowersWeekly = "weekly"
	// XStatisticsFollowersHourly and XStatisticsFollowers6h are for
	// OmniSocials Connect keys only.
	XStatisticsFollowersHourly = "hourly"
	XStatisticsFollowers6h     = "6h"
)

// XStatisticsPostSchedule is a custom check schedule (OmniSocials Connect
// keys, with Posts "custom"). At holds the times after publishing, such as
// ["1h", "3h", "1d", "7d"] (at most 24, 1h to 365d, at least 1 hour apart);
// ThenEvery repeats after the last one (1d to 30d); Until stops the
// repeats after this age.
type XStatisticsPostSchedule struct {
	At        []string `json:"at"`
	ThenEvery string   `json:"then_every,omitempty"`
	Until     string   `json:"until,omitempty"`
}

// AccountXStatistics is Account.XStatistics: the X statistics setting of an
// X account.
type AccountXStatistics struct {
	// Posts is "full", "light", "off" or "custom".
	Posts string `json:"posts"`
	// Followers is "daily", "weekly", "off", "hourly" or "6h".
	Followers string `json:"followers"`
	// PostSchedule is the custom schedule when Posts is "custom", else nil.
	PostSchedule *XStatisticsPostSchedule `json:"post_schedule"`
	// Source is where the setting comes from: "account", "customer",
	// "partner_default" or "default".
	Source string `json:"source"`
	// Billing is "credits" (the organisation's credits) or "own_x_app" (an
	// OmniSocials Connect partner's own X app pays).
	Billing string `json:"billing"`
	// State is "active", "paused" (the organisation ran out of credits) or
	// "off" (posts and followers both off).
	State string `json:"state"`
	// PausedReason is "insufficient_credits" while paused, else nil.
	PausedReason *string `json:"paused_reason"`
}

// XStatisticsInput is the X statistics change of Accounts.Update. Set one
// field or more; an empty field is left out and keeps its value.
type XStatisticsInput struct {
	Posts     string `json:"posts,omitempty"`
	Followers string `json:"followers,omitempty"`
	// PostSchedule is for OmniSocials Connect keys only, with Posts
	// "custom".
	PostSchedule *XStatisticsPostSchedule `json:"post_schedule,omitempty"`
}

// AccountUpdateParams is the request body of Accounts.Update.
type AccountUpdateParams struct {
	// XStatistics is the new X statistics setting. nil is sent as null
	// (OmniSocials Connect keys only): the account follows its customer's
	// setting again.
	XStatistics *XStatisticsInput `json:"x_statistics"`
}

// AccountsListResponse is the Accounts.List response envelope.
type AccountsListResponse struct {
	Data []Account `json:"data"`
	// WorkspaceID is the stable id of the active workspace this key belongs to.
	WorkspaceID int `json:"workspace_id,omitempty"`
	// WorkspaceName is the human-readable name of the active workspace.
	WorkspaceName *string `json:"workspace_name,omitempty"`
	WorkspaceIcon *string `json:"workspace_icon,omitempty"`
}

// List calls `GET /accounts`: the workspace's connected social accounts.
func (s *AccountsService) List(ctx context.Context) (*AccountsListResponse, error) {
	var out AccountsListResponse
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

// Update calls `PATCH /accounts/:id`: change how often the X statistics of
// an X account are read. X bills every read and OmniSocials passes X's price
// on as credits. Needs the opt-in scope accounts:write and billing rights
// (manage_billing) for the key's creator. With an OmniSocials Connect key
// the partner's own X app pays: Posts also takes "custom" with
// PostSchedule, Followers also "hourly" and "6h", and a nil XStatistics
// (sent as null) gives the account back to its customer's setting. A nil
// params sends an empty body (the API answers 400 validation_error).
// Returns the account with its new XStatistics. Errors: 400
// validation_error, schedule_invalid (the message says why), not_supported
// (not an X account); 403 forbidden; 404 not_found.
func (s *AccountsService) Update(ctx context.Context, id string, params *AccountUpdateParams) (*ItemResponse[Account], error) {
	var out ItemResponse[Account]
	if err := s.client.patch(ctx, "/accounts/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
