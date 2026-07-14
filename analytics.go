package omnisocials

import (
	"context"
	"net/url"
	"strings"
)

// AnalyticsService covers the /analytics endpoints.
type AnalyticsService struct {
	client *Client
}

// PostAnalyticsPlatformEntry is one platform's latest metrics for a post. For
// thread platforms (X, Bluesky, Mastodon) Metrics is summed across all parts
// and PlatformPostID is the thread root; ThreadParts is 1 for single posts.
type PostAnalyticsPlatformEntry struct {
	Platform       string         `json:"platform"`
	PlatformPostID string         `json:"platform_post_id,omitempty"`
	Metrics        map[string]any `json:"metrics"`
	ThreadParts    int            `json:"thread_parts,omitempty"`
	CollectedAt    string         `json:"collected_at,omitempty"`
}

// PostAnalytics is the per-post analytics payload: one entry per platform the
// post was published to.
type PostAnalytics struct {
	PostID    string                                `json:"post_id"`
	Platforms map[string]PostAnalyticsPlatformEntry `json:"platforms"`
}

// PostAnalyticsBatchResponse is the Analytics.Posts response.
type PostAnalyticsBatchResponse struct {
	Data  []PostAnalytics `json:"data"`
	Count int             `json:"count,omitempty"`
}

// AnalyticsPlatformBreakdown is one platform's slice of the overview.
type AnalyticsPlatformBreakdown struct {
	Posts             int     `json:"posts"`
	TotalEngagement   float64 `json:"total_engagement"`
	TotalImpressions  float64 `json:"total_impressions"`
	AverageEngagement float64 `json:"average_engagement"`
	EngagementRate    float64 `json:"engagement_rate"`
}

// AnalyticsOverview is the workspace-wide totals for a period.
type AnalyticsOverview struct {
	TotalPosts            int                                   `json:"total_posts"`
	TotalPlatforms        int                                   `json:"total_platforms"`
	TotalEngagement       float64                               `json:"total_engagement"`
	TotalImpressions      float64                               `json:"total_impressions"`
	AverageEngagementRate float64                               `json:"average_engagement_rate"`
	TopPerformingPlatform *string                               `json:"top_performing_platform,omitempty"`
	PlatformBreakdown     map[string]AnalyticsPlatformBreakdown `json:"platform_breakdown,omitempty"`
}

// AnalyticsOverviewResponse is the Analytics.Overview response envelope.
type AnalyticsOverviewResponse struct {
	Data AnalyticsOverview `json:"data"`
	// Period echoes the resolved period (either the named period or
	// "YYYY-MM-DD to YYYY-MM-DD").
	Period    string `json:"period,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// AnalyticsOverviewParams filters Analytics.Overview. Use either Period, or
// StartDate + EndDate.
type AnalyticsOverviewParams struct {
	// Period is a named period, e.g. "7d", "30d", "90d".
	Period string
	// StartDate is an ISO date (YYYY-MM-DD); use with EndDate instead of
	// Period.
	StartDate string
	// EndDate is an ISO date (YYYY-MM-DD); use with StartDate instead of
	// Period.
	EndDate string
}

// AccountAnalyticsEntry is one account-level stats snapshot.
type AccountAnalyticsEntry struct {
	Platform          string         `json:"platform"`
	PlatformAccountID string         `json:"platform_account_id,omitempty"`
	Date              string         `json:"date,omitempty"`
	Metrics           map[string]any `json:"metrics"`
	CollectedAt       string         `json:"collected_at,omitempty"`
}

// AccountAnalyticsResponse is the Analytics.Accounts response.
type AccountAnalyticsResponse struct {
	Data []AccountAnalyticsEntry `json:"data"`
	// Date is the resolved snapshot date.
	Date string `json:"date,omitempty"`
}

// AccountAnalyticsParams filters Analytics.Accounts.
type AccountAnalyticsParams struct {
	// Platform restricts to one platform, e.g. "instagram".
	Platform string
	// Date is an ISO date (YYYY-MM-DD) to fetch a specific day's snapshot.
	Date string
}

// BestTimesParams is the input for Analytics.BestTimes.
type BestTimesParams struct {
	// Platform is required, e.g. "instagram".
	Platform string
	// Timezone is the IANA timezone for the returned slots, e.g.
	// "Europe/Amsterdam".
	Timezone string
}

// Post calls `GET /analytics/posts/:id`: the latest per-platform metrics for
// one post.
func (s *AnalyticsService) Post(ctx context.Context, postID string) (*ItemResponse[PostAnalytics], error) {
	var out ItemResponse[PostAnalytics]
	if err := s.client.get(ctx, "/analytics/posts/"+url.PathEscape(postID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Posts calls `GET /analytics/posts?ids=a,b,c`: batch metrics for up to 100
// posts in one call instead of one request per post.
func (s *AnalyticsService) Posts(ctx context.Context, postIDs []string) (*PostAnalyticsBatchResponse, error) {
	query := url.Values{}
	query.Set("ids", strings.Join(postIDs, ","))
	var out PostAnalyticsBatchResponse
	if err := s.client.get(ctx, "/analytics/posts", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Overview calls `GET /analytics/overview`: workspace-wide totals for a
// period.
func (s *AnalyticsService) Overview(ctx context.Context, params *AnalyticsOverviewParams) (*AnalyticsOverviewResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Period != "" {
			query.Set("period", params.Period)
		}
		if params.StartDate != "" {
			query.Set("start_date", params.StartDate)
		}
		if params.EndDate != "" {
			query.Set("end_date", params.EndDate)
		}
	}
	var out AnalyticsOverviewResponse
	if err := s.client.get(ctx, "/analytics/overview", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Accounts calls `GET /analytics/accounts`: account-level stats (followers
// etc).
func (s *AnalyticsService) Accounts(ctx context.Context, params *AccountAnalyticsParams) (*AccountAnalyticsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.Date != "" {
			query.Set("date", params.Date)
		}
	}
	var out AccountAnalyticsResponse
	if err := s.client.get(ctx, "/analytics/accounts", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BestTimes calls `GET /analytics/best-times`: recommended posting time slots
// for a platform, based on when the account's audience engages. Returned as a
// map passthrough (`data` holds the platform, timezone, and scored slots).
func (s *AnalyticsService) BestTimes(ctx context.Context, params *BestTimesParams) (map[string]any, error) {
	query := url.Values{}
	if params != nil {
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.Timezone != "" {
			query.Set("timezone", params.Timezone)
		}
	}
	var out map[string]any
	if err := s.client.get(ctx, "/analytics/best-times", query, &out); err != nil {
		return nil, err
	}
	return out, nil
}
