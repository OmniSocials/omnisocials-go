package omnisocials

import "context"

// CreditsService covers GET /credits: the organisation's credit balance and
// what X statistics cost. Top-ups are made in the OmniSocials app (Settings
// -> Organisation -> Billing -> Credits).
type CreditsService struct {
	client *Client
}

// Credits is the data of Credits.Get. 1 credit = EUR 0.01.
type Credits struct {
	Balance    float64           `json:"balance"`
	AutoRefill CreditsAutoRefill `json:"auto_refill"`
	// ThisMonth is the credits used this month per reason (for example
	// "x_statistics", "x_url_post", "x_dm").
	ThisMonth   map[string]float64 `json:"this_month"`
	XStatistics CreditsXStatistics `json:"x_statistics"`
}

// CreditsAutoRefill says whether auto-refill is on.
type CreditsAutoRefill struct {
	Enabled bool `json:"enabled"`
}

// CreditsXStatistics is what X statistics cost the organisation.
type CreditsXStatistics struct {
	// State is "active" or "paused" (the balance reached 0; reads stop
	// until credits are added).
	State    string  `json:"state"`
	PausedAt *string `json:"paused_at"`
	// Charging says whether X statistics are charged yet, and from when.
	Charging CreditsXStatisticsCharging `json:"charging"`
	// Prices are the credits per read.
	Prices CreditsXStatisticsPrices `json:"prices"`
	// TotalCreditsMonth is the estimate for a month over every X account.
	TotalCreditsMonth float64                     `json:"total_credits_month"`
	ThisMonth         CreditsXStatisticsUsage     `json:"this_month"`
	Accounts          []CreditsXStatisticsAccount `json:"accounts"`
}

// CreditsXStatisticsCharging says whether X statistics are charged yet.
type CreditsXStatisticsCharging struct {
	Active bool    `json:"active"`
	From   *string `json:"from"`
}

// CreditsXStatisticsPrices are the credits per read: PostRead 0.5,
// MediaRead 0.5 (a video or GIF), FollowerRead 1.
type CreditsXStatisticsPrices struct {
	PostRead     float64 `json:"post_read"`
	MediaRead    float64 `json:"media_read"`
	FollowerRead float64 `json:"follower_read"`
}

// CreditsXStatisticsUsage is this month's reads and the credits they used.
type CreditsXStatisticsUsage struct {
	PostReads     int     `json:"post_reads"`
	MediaReads    int     `json:"media_reads"`
	FollowerReads int     `json:"follower_reads"`
	Refreshes     int     `json:"refreshes"`
	Credits       float64 `json:"credits"`
}

// CreditsXStatisticsAccount is one X account of the organisation with its
// setting and its estimate for a month.
type CreditsXStatisticsAccount struct {
	// AccountID is the account id as in Accounts.List ("<workspace_id>_x").
	AccountID   string  `json:"account_id"`
	WorkspaceID int64   `json:"workspace_id"`
	Handle      *string `json:"handle"`
	Posts       string  `json:"posts"`
	Followers   string  `json:"followers"`
	// Tweets30d is the tweets published in the last 30 days.
	Tweets30d int `json:"tweets_30d"`
	// CreditsMonth is the estimate for a month at this setting.
	CreditsMonth float64 `json:"credits_month"`
}

// Get calls `GET /credits`: the balance, auto-refill, the credits used this
// month and the X statistics overview. Needs the opt-in scope credits:read
// and billing rights (manage_billing) for the key's creator; OmniSocials
// Connect keys get 403 not_available_for_connect.
func (s *CreditsService) Get(ctx context.Context) (*ItemResponse[Credits], error) {
	var out ItemResponse[Credits]
	if err := s.client.get(ctx, "/credits", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
