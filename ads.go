package omnisocials

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// AdsService covers the /ads endpoints: boost a published post or create a
// new ad on Meta, Pinterest, TikTok, LinkedIn, X and Google.
//
// Ads spend real money once a campaign starts. Boosts and drafts are created
// as drafts (no spend). Starting or raising spend needs a key with the
// opt-in ads:spend scope ("Start ads", a second switch under "Ads"; its
// creator needs manage_ads): Activate, ActivateMulti, Resume, Drafts.Launch,
// a budget raise or any change of a campaign that reached the network
// through Update, and the parity calls that switch something on or raise a
// budget (see ads_parity.go). Each repeats the amount: show the person the
// amount and get their yes first. Without ads:spend these calls answer 403
// start_in_app with the page in the app where a person does it; on Google
// (AdCapabilities.APIDraftOnly) they always do, also with ads:spend. Pause
// and Delete stay open, except on the networks in
// AdCapabilities.APIStopInApp (Google). The calls that start or raise spend
// are not retried after a 5xx or a connection error (only a 429 is),
// because the first attempt may have started the ad: read the campaign or
// the draft to see what happened. Ads errors may carry `param` (the field at
// fault): read it from APIError.Body (`error.param`).
//
// Budgets are in major currency units (10 = 10 EUR) in the ad account's
// currency. Ad accounts are connected and picked in the OmniSocials
// dashboard. The scopes are opt-in on the API key (the "Ads" switch):
// ads:read for the reads, ads:write for the rest. A key without them gets a
// 403 insufficient_scope. ads:spend (the "Start ads" switch under it) is
// only for the calls that start or raise spend (see above).
type AdsService struct {
	client *Client

	// Drafts covers /ads/drafts: "Create an ad" drafts, one campaign per
	// network with the same ads. A draft never spends; Drafts.Launch starts
	// it (a key with ads:spend), or a person does in the app from its
	// ReviewURL.
	Drafts *AdDraftsService

	// The Ads API parity sub-services (2026-11-05, ads_parity.go).

	// Objects covers the campaigns, ad sets and ads of an ad account (also
	// the ones made outside OmniSocials) and their import.
	Objects *AdObjectsService
	// CampaignAds covers the ads of an OmniSocials campaign.
	CampaignAds *AdCampaignAdsService
	// Comments covers the comments on ads.
	Comments *AdCommentsService
	// Targeting covers targeting search, browse, reach estimates and bid
	// suggestions.
	Targeting *AdTargetingService
	// Audiences covers custom audiences.
	Audiences *AdAudiencesService
	// LeadForms covers lead forms.
	LeadForms *AdLeadFormsService
	// Leads covers the leads copied from the lead forms.
	Leads *AdLeadsService
	// Conversions covers pixels, conversion goals, custom conversions and
	// server-side events.
	Conversions *AdConversionsService
	// Insights covers live numbers, campaign and ad analytics, the
	// timeline, rankings and alerts.
	Insights *AdInsightsService
	// Reports covers background reports.
	Reports *AdReportsService
	// Rules covers automated rules.
	Rules *AdRulesService
	// Results covers your own numbers per campaign and day.
	Results *AdResultsService
	// Media covers the network's own image and video library.
	Media *AdMediaService
	// Previews covers previews drawn by the network.
	Previews *AdPreviewsService
	// Partnership covers partnership ads (Meta).
	Partnership *AdPartnershipService
	// Accounts covers ad account admin (health, finance, settings, people,
	// change log, businesses, DSA, hierarchy, identities).
	Accounts *AdAccountsService
	// ReachFrequency covers Meta reach and frequency.
	ReachFrequency *AdReachFrequencyService
	// Studies covers A/B tests and lift studies (Meta).
	Studies *AdStudiesService
	// BudgetSchedules covers planned budget increases (Meta).
	BudgetSchedules *AdBudgetSchedulesService
	// Google covers the Google reports.
	Google *AdGoogleService
}

// AdDraftsService covers the /ads/drafts endpoints. Reach it through
// client.Ads.Drafts.
type AdDraftsService struct {
	client *Client
}

// ---- Ids ---------------------------------------------------------------------

// AdID is the id of an ad campaign or an ad account. The API sends it as a
// number or as a numeric string; both decode. It encodes as a JSON number.
type AdID int64

// UnmarshalJSON reads a number or a numeric string. A JSON null leaves the
// id unchanged.
func (id *AdID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("omnisocials: ad id %s is not a number", data)
	}
	v, err := n.Int64()
	if err != nil {
		return fmt.Errorf("omnisocials: ad id %s is not a number", data)
	}
	*id = AdID(v)
	return nil
}

// String returns the id in decimal.
func (id AdID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// ---- Shared shapes -------------------------------------------------------------

// AdBudget is a campaign budget. Amount is in MAJOR currency units (10 = 10
// EUR); network minimums apply (for example TikTok 20 a day). Currency is
// the 3-letter ISO code and must match the ad account's currency.
type AdBudget struct {
	// Type is "daily" (default) or "lifetime" (the whole schedule). A draft
	// budget is always daily: leave Type empty there.
	Type     string  `json:"type,omitempty"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// AdSchedule is when a campaign runs (ISO 8601 datetimes).
type AdSchedule struct {
	StartAt string `json:"start_at,omitempty"`
	EndAt   string `json:"end_at,omitempty"`
}

// AdCreative holds the network-specific creative fields of a boost.
// TikTok (Spark Ads) needs SparkAuthCode: the post owner makes it in the
// TikTok app (video, "...", Ad settings, Ad authorization). Google (Demand
// Gen, YouTube videos) needs Headlines, Descriptions and LogoURL. Meta,
// Pinterest, X and LinkedIn boosts need none.
type AdCreative struct {
	SparkAuthCode string `json:"spark_auth_code,omitempty"`
	// Headlines: Google, 1 to 5 of up to 40 characters.
	Headlines []string `json:"headlines,omitempty"`
	// Descriptions: Google, 1 to 5 of up to 90 characters.
	Descriptions []string `json:"descriptions,omitempty"`
	// LogoURL: Google, a square logo picture.
	LogoURL       string   `json:"logo_url,omitempty"`
	LongHeadlines []string `json:"long_headlines,omitempty"`
	BusinessName  string   `json:"business_name,omitempty"`
	// Link: TikTok traffic boosts, the website the button opens (required
	// there).
	Link           string `json:"link,omitempty"`
	LandingPageURL string `json:"landing_page_url,omitempty"`
}

// ---- Ad accounts -----------------------------------------------------------------

// AdAccountInfo is the display info of an ad account, from the network.
type AdAccountInfo struct {
	Name string `json:"name,omitempty"`
	// Currency is the billing currency; campaign budgets must use it.
	Currency string `json:"currency,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	// BillingOK is false when no payment method is set on the network; the
	// ads do not run until the person adds one in the network's Ads
	// Manager. Nil when the network does not say.
	BillingOK *bool `json:"billing_ok,omitempty"`
}

// AdAccount is a connected ad account (Ads.ListAccounts).
type AdAccount struct {
	// ID is the OmniSocials ad account id (use it as AdAccountID).
	ID AdID `json:"id"`
	// Platform is "meta", "pinterest", "tiktok", "linkedin", "x" or
	// "google".
	Platform string `json:"platform"`
	// ExternalAccountID is the network's own ad account id (Meta act_...).
	ExternalAccountID string         `json:"external_account_id"`
	AccountInfo       *AdAccountInfo `json:"account_info,omitempty"`
	// IsSelected reports whether a person picked this account for spending
	// in the dashboard. Only selected accounts can run campaigns.
	IsSelected  bool   `json:"is_selected"`
	Status      string `json:"status"`
	NeedsReauth bool   `json:"needs_reauth,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// AdAccountsResponse is the Ads.ListAccounts response.
type AdAccountsResponse struct {
	Data []AdAccount `json:"data"`
	// PlatformsAvailable are the ads networks this server supports.
	PlatformsAvailable []string `json:"platforms_available,omitempty"`
}

// ---- Campaigns ---------------------------------------------------------------------

// AdCampaign is an ad campaign or a boost.
type AdCampaign struct {
	ID       AdID   `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	// CampaignType is "campaign" or "boost".
	CampaignType string `json:"campaign_type"`
	// Objective is "engagement", "traffic", "awareness", "video_views",
	// "lead_generation", "conversions", "messages" or "calls".
	Objective string `json:"objective,omitempty"`
	// Status is "draft", "pending_review" (in the network's ad review),
	// "active", "rejected", "paused", "completed" or "error". A campaign is
	// created as "draft" and spends only once a person starts it in the app.
	Status string    `json:"status"`
	Budget *AdBudget `json:"budget,omitempty"`
	// AdAccountID is the ad account it runs on; nil on a draft without one.
	AdAccountID *AdID `json:"ad_account_id,omitempty"`
	// Targeting is the unified targeting (countries, ages, genders...) plus
	// "network", the network's own targeting (AdNetworkTargeting);
	// raw_targeting passes a native spec through.
	Targeting map[string]any `json:"targeting,omitempty"`
	// Settings are the network settings (bidding, conversion goal, lead
	// form, messaging).
	Settings *AdNetworkSettings `json:"settings,omitempty"`
	Schedule *AdSchedule        `json:"schedule,omitempty"`
	// Metrics are the latest synced numbers: spend (major units),
	// impressions, clicks, ctr.
	Metrics map[string]any `json:"metrics,omitempty"`
	// ExternalCampaignID is the network's campaign id; nil while a draft.
	ExternalCampaignID *string `json:"external_campaign_id,omitempty"`
	// Errors is the readable reason when the status is "error" or
	// "rejected".
	Errors *string `json:"errors,omitempty"`
	// GroupID is set on a campaign of a "Create an ad" draft: start it with
	// Ads.Drafts.Launch (a key with ads:spend) or in the app.
	GroupID      *string `json:"group_id,omitempty"`
	LastSyncedAt *string `json:"last_synced_at,omitempty"`
	// CreatedAt and UpdatedAt arrive in camelCase on a campaign
	// (`createdAt`, `updatedAt`), unlike AdAccount.CreatedAt.
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// AdCampaignListParams filters Ads.List.
type AdCampaignListParams struct {
	// Status is one of the AdCampaign statuses.
	Status string
	// Platform is "meta", "pinterest", "tiktok", "linkedin", "x" or
	// "google".
	Platform string
	// CampaignType is "campaign" or "boost".
	CampaignType string
	// Limit is 1 to 100 (default 25).
	Limit int
	// Offset is the number of campaigns to skip.
	Offset int
}

// AdBoostParams is the request body for Ads.Boost.
type AdBoostParams struct {
	// PostID is the OmniSocials id of a published post.
	PostID string `json:"post_id"`
	// AdAccountID is from Ads.ListAccounts, picked in the dashboard.
	AdAccountID AdID     `json:"ad_account_id"`
	Budget      AdBudget `json:"budget"`
	// Objective is "engagement", "traffic", "awareness" or "video_views".
	Objective string         `json:"objective,omitempty"`
	Targeting map[string]any `json:"targeting,omitempty"`
	Schedule  *AdSchedule    `json:"schedule,omitempty"`
	Creative  *AdCreative    `json:"creative,omitempty"`
	Name      string         `json:"name,omitempty"`
}

// AdBoostMultiParams is the request body for Ads.BoostMulti: one draft boost
// per ad account (up to 10), each with the SAME budget.
type AdBoostMultiParams struct {
	PostID       string         `json:"post_id"`
	AdAccountIDs []AdID         `json:"ad_account_ids"`
	Budget       AdBudget       `json:"budget"`
	Objective    string         `json:"objective,omitempty"`
	Targeting    map[string]any `json:"targeting,omitempty"`
	Schedule     *AdSchedule    `json:"schedule,omitempty"`
	Creative     *AdCreative    `json:"creative,omitempty"`
	Name         string         `json:"name,omitempty"`
}

// AdCampaignUpdateParams is the request body for Ads.Update. Only set
// fields are sent. A draft changes with ads:write. A campaign that reached
// the network changes only with a key that has ads:spend (never on Google);
// without it the call answers 403 start_in_app.
type AdCampaignUpdateParams struct {
	Name   string    `json:"name,omitempty"`
	Budget *AdBudget `json:"budget,omitempty"`
	// ConfirmBudgetAmount is required to RAISE the budget of a running
	// campaign (a key with ads:spend, outside APIDraftOnly): the new amount
	// again, after the person agreed to it.
	ConfirmBudgetAmount float64        `json:"confirm_budget_amount,omitempty"`
	Targeting           map[string]any `json:"targeting,omitempty"`
	Schedule            *AdSchedule    `json:"schedule,omitempty"`
	// Objective and Settings change only while the campaign is a draft (400
	// settings_locked after the start: duplicate it with Ads.Duplicate).
	Objective string             `json:"objective,omitempty"`
	Settings  *AdNetworkSettings `json:"settings,omitempty"`
}

// AdMultiResponse is the `{ results }` answer of Ads.BoostMulti and
// Ads.ActivateMulti (no `data` envelope).
type AdMultiResponse[T any] struct {
	Results []T `json:"results"`
}

// AdMultiBoostResult is one ad account of Ads.BoostMulti. One failed
// account never stops the others.
type AdMultiBoostResult struct {
	AdAccountID AdID        `json:"ad_account_id"`
	Platform    *string     `json:"platform,omitempty"`
	OK          bool        `json:"ok"`
	Campaign    *AdCampaign `json:"campaign,omitempty"`
	Error       string      `json:"error,omitempty"`
	Code        string      `json:"code,omitempty"`
}

// AdMultiActivateResult is one campaign of Ads.ActivateMulti. Code
// "start_in_app" = the person starts it in the app (Error names the page).
type AdMultiActivateResult struct {
	CampaignID AdID        `json:"campaign_id"`
	OK         bool        `json:"ok"`
	Campaign   *AdCampaign `json:"campaign,omitempty"`
	Error      string      `json:"error,omitempty"`
	Code       string      `json:"code,omitempty"`
}

// AdDeleted is the answer of Ads.Delete and Ads.Drafts.Delete.
type AdDeleted struct {
	Deleted bool `json:"deleted"`
}

// ---- Overview, suggestions, keyword research ------------------------------------

// AdsTotals are the summed numbers of every campaign, spend in major units.
type AdsTotals struct {
	// Spend is nil when the campaigns use more than one currency: read
	// AdsOverview.TotalsByCurrency then. Spend is never added across
	// currencies.
	Spend *float64 `json:"spend"`
	// Currency is the one currency of Spend, nil when there are several
	// (or none).
	Currency    *string `json:"currency"`
	Impressions float64 `json:"impressions"`
	Clicks      float64 `json:"clicks"`
}

// AdsCurrencyTotals are the numbers of the campaigns in one currency.
type AdsCurrencyTotals struct {
	// Currency is the 3-letter ISO code, nil for campaigns without one.
	Currency    *string `json:"currency"`
	Spend       float64 `json:"spend"`
	Impressions float64 `json:"impressions"`
	Clicks      float64 `json:"clicks"`
}

// AdsOverview is the Ads.Overview payload.
type AdsOverview struct {
	ConnectedAccounts int            `json:"connected_accounts"`
	CampaignsTotal    int            `json:"campaigns_total"`
	CampaignsByStatus map[string]int `json:"campaigns_by_status"`
	Totals            AdsTotals      `json:"totals"`
	// TotalsByCurrency holds spend, impressions and clicks per currency.
	TotalsByCurrency []AdsCurrencyTotals `json:"totals_by_currency"`
}

// BoostSuggestion is an organic post that does better than the workspace
// median: a boost candidate.
type BoostSuggestion struct {
	PostID          string  `json:"post_id"`
	Platform        string  `json:"platform"`
	PublishedAt     string  `json:"published_at"`
	MetricValue     float64 `json:"metric_value"`
	WorkspaceMedian float64 `json:"workspace_median"`
	// Multiplier is how many times the median.
	Multiplier         float64 `json:"multiplier"`
	Reason             string  `json:"reason"`
	DescriptionSnippet *string `json:"description_snippet,omitempty"`
	ThumbnailURL       *string `json:"thumbnail_url,omitempty"`
}

// AdKeywordMonth is the search volume of one month.
type AdKeywordMonth struct {
	Year     *int  `json:"year"`
	Month    *int  `json:"month"`
	Searches int64 `json:"searches"`
}

// AdKeywordIdea is one keyword with its Google search numbers.
type AdKeywordIdea struct {
	Keyword string `json:"keyword"`
	// AvgMonthlySearches is the average over the last 12 months.
	AvgMonthlySearches int64 `json:"avg_monthly_searches"`
	// Competition is "low", "medium", "high" or "unspecified".
	Competition string `json:"competition"`
	// CompetitionIndex is 0 to 100.
	CompetitionIndex *int `json:"competition_index"`
	// LowTopOfPageBid and HighTopOfPageBid are the top-of-page bid range in
	// the ad account's currency (major units).
	LowTopOfPageBid  *float64         `json:"low_top_of_page_bid"`
	HighTopOfPageBid *float64         `json:"high_top_of_page_bid"`
	MonthlySearches  []AdKeywordMonth `json:"monthly_searches"`
	// CloseVariants is set on Ads.KeywordMetrics only.
	CloseVariants []string `json:"close_variants,omitempty"`
}

// AdKeywordResearchParams is the request body for Ads.ResearchKeywords.
// Give Keywords, URL or both.
type AdKeywordResearchParams struct {
	// Keywords are seed keywords, up to 20.
	Keywords []string `json:"keywords,omitempty"`
	// URL is a landing page or homepage to take ideas from.
	URL string `json:"url,omitempty"`
	// Country is ISO 3166-1 alpha-2, default "US".
	Country string `json:"country,omitempty"`
	// Language is ISO 639-1, default "en".
	Language string `json:"language,omitempty"`
	// Limit is 1 to 500, default 100.
	Limit int `json:"limit,omitempty"`
	// AdAccountID is only needed when the workspace has more than one
	// Google Ads account.
	AdAccountID AdID `json:"ad_account_id,omitempty"`
}

// AdKeywordSeed echoes the seed of a keyword research call.
type AdKeywordSeed struct {
	Keywords []string `json:"keywords"`
	URL      *string  `json:"url"`
}

// AdKeywordResearchResult is the Ads.ResearchKeywords payload.
type AdKeywordResearchResult struct {
	AdAccountID AdID            `json:"ad_account_id"`
	Country     string          `json:"country"`
	Language    string          `json:"language"`
	Seed        AdKeywordSeed   `json:"seed"`
	Total       int             `json:"total"`
	Cached      bool            `json:"cached"`
	Ideas       []AdKeywordIdea `json:"ideas"`
}

// AdKeywordMetricsParams is the request body for Ads.KeywordMetrics.
type AdKeywordMetricsParams struct {
	// Keywords are the exact keywords to measure.
	Keywords    []string `json:"keywords"`
	Country     string   `json:"country,omitempty"`
	Language    string   `json:"language,omitempty"`
	AdAccountID AdID     `json:"ad_account_id,omitempty"`
}

// AdKeywordMetricsResult is the Ads.KeywordMetrics payload.
type AdKeywordMetricsResult struct {
	AdAccountID AdID            `json:"ad_account_id"`
	Country     string          `json:"country"`
	Language    string          `json:"language"`
	Total       int             `json:"total"`
	Cached      bool            `json:"cached"`
	Keywords    []AdKeywordIdea `json:"keywords"`
}

// ---- Capabilities ------------------------------------------------------------------

// AdGoalSupport says how a network handles one goal.
type AdGoalSupport struct {
	// OK is true, false or "approx" (the network optimises for something
	// close to the goal; Note says what).
	OK         any    `json:"ok"`
	Note       string `json:"note,omitempty"`
	Reason     string `json:"reason,omitempty"`
	NeedsVideo bool   `json:"needsVideo,omitempty"`
	// APIOnly marks a goal through the API and MCP only (lead_generation,
	// conversions, messages, calls).
	APIOnly bool `json:"apiOnly,omitempty"`
	// Needs says what the goal needs: "lead_form", "conversion",
	// "messaging", "phone_number".
	Needs []string `json:"needs,omitempty"`
}

// AdTextLimit is the limit of one text field on a network.
type AdTextLimit struct {
	Max      int  `json:"max"`
	Shown    int  `json:"shown"`
	Required bool `json:"required"`
	// Field is false when the network has no such field.
	Field   *bool `json:"field,omitempty"`
	NoEmoji bool  `json:"noEmoji,omitempty"`
}

// AdMinDailyBudget is a network's minimum daily budget, in major units.
type AdMinDailyBudget struct {
	Default    float64            `json:"default"`
	ByCurrency map[string]float64 `json:"byCurrency,omitempty"`
}

// AdNetworkCapabilities is what one network accepts for a new ad.
type AdNetworkCapabilities struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	RunsOn   string `json:"runs_on"`
	Identity string `json:"identity"`
	// Goals per objective key.
	Goals map[string]AdGoalSupport `json:"goals"`
	// Media holds the picture and video rules (formats, sizes, ratios).
	Media map[string]any `json:"media"`
	// Text per field: "primary_text", "headline", "description".
	Text map[string]AdTextLimit `json:"text"`
	// CTAs are the button keys the network takes.
	CTAs              []string         `json:"ctas"`
	NoCTA             bool             `json:"no_cta"`
	LinkRequired      bool             `json:"link_required,omitempty"`
	RequiresCountries bool             `json:"requires_countries,omitempty"`
	ExtraFields       []string         `json:"extra_fields"`
	MinDailyBudget    AdMinDailyBudget `json:"min_daily_budget"`
	MinDurationDays   *int             `json:"min_duration_days,omitempty"`
	// BoostGoals are the goals a boost can use on this network.
	BoostGoals []string `json:"boost_goals,omitempty"`
	// SpendNote says how far a day's spend can go over the daily amount.
	SpendNote         string `json:"spend_note,omitempty"`
	MaxAdsPerCampaign *int   `json:"max_ads_per_campaign,omitempty"`
	Review            string `json:"review"`
	// BidStrategies are the strategies AdBiddingSetting takes here.
	BidStrategies []string `json:"bid_strategies,omitempty"`
	// Settings and AdFields list the keys of AdNetworkSettings and of
	// AdDraftAd.Network[network] this network reads, with a short note.
	Settings map[string]string `json:"settings,omitempty"`
	AdFields map[string]string `json:"ad_fields,omitempty"`
}

// AdCTA is one button: its key and the label people see.
type AdCTA struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// AdCapabilities is the Ads.Capabilities payload.
type AdCapabilities struct {
	Networks []AdNetworkCapabilities `json:"networks"`
	Goals    []string                `json:"goals"`
	CTAs     []AdCTA                 `json:"ctas"`
	// CreateNetworks are the networks open to your organisation for new
	// ads.
	CreateNetworks []string `json:"create_networks"`
	// NetworkStates per network: "live", "preview" (your organisation only)
	// or "coming_soon".
	NetworkStates map[string]string `json:"network_states,omitempty"`
	// APIDraftOnly lists the networks where every API caller stops at the
	// draft, also a key with ads:spend (Google): a person starts and changes
	// those ads in the app. The MCP stops at the draft on every network.
	APIDraftOnly []string `json:"api_draft_only,omitempty"`
	// APIStopInApp lists the networks where the API may not even pause or
	// delete a campaign that reached the network (Google).
	APIStopInApp []string `json:"api_stop_in_app,omitempty"`
	// Features per network: the feature keys its API offers through
	// OmniSocials (a missing one answers 400 not_supported_on_network).
	Features      map[string][]string `json:"features,omitempty"`
	FeatureLabels map[string]string   `json:"feature_labels,omitempty"`
	// TargetingTypes per network: the types of Targeting.Search and the
	// keys of AdNetworkTargeting.Include and Exclude.
	TargetingTypes map[string][]string `json:"targeting_types,omitempty"`
	// APIGoals are the goals through the API and MCP only.
	APIGoals []string `json:"api_goals,omitempty"`
}

// ---- Drafts ------------------------------------------------------------------------

// AdMediaFile is one picture or video of a draft ad.
type AdMediaFile struct {
	URL string `json:"url"`
	// Type is "image" or "video".
	Type   string `json:"type"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Duration is in seconds, for a video.
	Duration float64 `json:"duration,omitempty"`
	// ThumbnailURL is the cover picture of a video.
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// AdPlacementMedia holds extra files for one kind of placement.
type AdPlacementMedia struct {
	// StoriesReels is Meta only, optional: a second picture or video for
	// Stories and Reels, upright 9:16 (1080 x 1920), the same kind as the
	// main file. Without it Meta fits the main file. Not for the
	// engagement goal.
	StoriesReels *AdMediaFile `json:"stories_reels,omitempty"`
}

// AdDraftAd is one ad of a draft: written once, mapped onto every network.
type AdDraftAd struct {
	Key         string        `json:"key,omitempty"`
	Name        string        `json:"name,omitempty"`
	Media       []AdMediaFile `json:"media"`
	PrimaryText string        `json:"primary_text"`
	// PrimaryTextB is a second text: the network tests A against B (a
	// second ad).
	PrimaryTextB string `json:"primary_text_b,omitempty"`
	Headline     string `json:"headline,omitempty"`
	Description  string `json:"description,omitempty"`
	// CTA is a button key from Ads.Capabilities.
	CTA  string `json:"cta,omitempty"`
	Link string `json:"link"`
	// Overrides per network replace the shared text, e.g.
	// {"x": {"primary_text": "..."}}.
	Overrides map[string]any `json:"overrides,omitempty"`
	// Network holds network-only fields, e.g. {"google": {"business_name":
	// "...", "logo": {"url": "..."}}}, {"meta": AdMetaAdFields{...}},
	// {"tiktok": AdTikTokAdFields{...}}, {"linkedin": {"lead_form_id":
	// "..."}} or Google's "responsive_search_ad" (AdResponsiveSearchAd).
	Network        map[string]any    `json:"network,omitempty"`
	PlacementMedia *AdPlacementMedia `json:"placement_media,omitempty"`
}

// AdDraftTargeting is who a draft reaches.
type AdDraftTargeting struct {
	// Countries are ISO country codes.
	Countries []string `json:"countries"`
	// AutomaticAudience true (the default) lets the network find the
	// audience (no age or gender limits).
	AutomaticAudience *bool `json:"automatic_audience,omitempty"`
	AgeMin            int   `json:"age_min,omitempty"`
	AgeMax            int   `json:"age_max,omitempty"`
	// Genders: "male", "female".
	Genders []string `json:"genders,omitempty"`
	// Placements is "automatic" (default) or "feeds".
	Placements string `json:"placements,omitempty"`
	// PoliticalIntent is LinkedIn's EU statement: "NOT_POLITICAL" or
	// "POLITICAL".
	PoliticalIntent string `json:"political_intent,omitempty"`
	// SpecialAdCategories and SpecialAdCategoryCountry are Meta's special
	// ad categories (housing, employment, financial products, ...).
	SpecialAdCategories      []string `json:"special_ad_categories,omitempty"`
	SpecialAdCategoryCountry []string `json:"special_ad_category_country,omitempty"`
	// RegionalIdentities is Meta's verified payer per regional category.
	RegionalIdentities map[string]any `json:"regional_identities,omitempty"`
	// Networks is each network's own targeting (interests, places, job
	// titles, custom audiences) with ids from Targeting.Search of that
	// network. A save without it keeps each network's own.
	Networks map[string]AdNetworkTargeting `json:"networks,omitempty"`
}

// AdDraftSchedule is how long a draft runs.
type AdDraftSchedule struct {
	// DurationDays is the number of days; nil runs the ad until it is
	// stopped.
	DurationDays *int `json:"duration_days"`
}

// AdDraftPlatform is one entry of AdDraftParams.Platforms with its own
// daily amount or ad account.
type AdDraftPlatform struct {
	Platform     string  `json:"platform"`
	BudgetAmount float64 `json:"budget_amount,omitempty"`
	AdAccountID  AdID    `json:"ad_account_id,omitempty"`
	// Settings are the network settings (bidding, conversion goal, lead
	// form, messaging).
	Settings *AdNetworkSettings `json:"settings,omitempty"`
}

// AdDraftParams is the request body for Ads.Drafts.Create and
// Ads.Drafts.Update (the full draft).
type AdDraftParams struct {
	Name string `json:"name,omitempty"`
	// Objective is "traffic", "engagement", "awareness" or "video_views";
	// through the API also "lead_generation", "conversions", "messages" or
	// "calls" (AdGoalSupport.Needs says what each network needs).
	Objective string `json:"objective"`
	// Platforms: a []string of network keys, or a []AdDraftPlatform (or a
	// mixed []any) for a daily amount or an ad account per network.
	Platforms any `json:"platforms"`
	// Budget is the daily amount per network (leave Type empty).
	Budget AdBudget `json:"budget"`
	// Schedule: nil leaves it out; &AdDraftSchedule{} runs until stopped.
	Schedule  *AdDraftSchedule `json:"schedule,omitempty"`
	Targeting AdDraftTargeting `json:"targeting"`
	Ads       []AdDraftAd      `json:"ads"`
}

// AdDraftIssue is one thing that stops a network or an ad from starting.
type AdDraftIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AdDraftAdReadiness is the readiness of one ad on one network.
type AdDraftAdReadiness struct {
	Key    string         `json:"key"`
	Index  int            `json:"index"`
	Ready  bool           `json:"ready"`
	Issues []AdDraftIssue `json:"issues"`
}

// AdDraftReadiness says what still stops a network of a draft.
type AdDraftReadiness struct {
	Ready  bool                 `json:"ready"`
	Issues []AdDraftIssue       `json:"issues"`
	Ads    []AdDraftAdReadiness `json:"ads"`
}

// AdDraftAccount is the ad account of a network in a draft (and in
// AdDraftDefaults.Accounts).
type AdDraftAccount struct {
	ID       AdID    `json:"id"`
	Name     *string `json:"name"`
	Currency *string `json:"currency"`
	// BillingOK is false when no payment method is set on the network; nil
	// when the network does not say.
	BillingOK   *bool `json:"billing_ok"`
	NeedsReauth bool  `json:"needs_reauth"`
}

// AdDraftCampaign is the campaign of one network in a draft.
type AdDraftCampaign struct {
	ID       AdID   `json:"id"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	// Editable is true while the network never started: a save changes it,
	// a launch starts it.
	Editable           bool            `json:"editable"`
	ExternalCampaignID *string         `json:"external_campaign_id"`
	Budget             *AdBudget       `json:"budget"`
	AdAccount          *AdDraftAccount `json:"ad_account"`
	Errors             *string         `json:"errors"`
	// Settings are the network settings of this campaign.
	Settings  *AdNetworkSettings `json:"settings,omitempty"`
	Readiness AdDraftReadiness   `json:"readiness"`
}

// AdCurrencyChange is set when a network's ad account uses another currency
// than the draft.
type AdCurrencyChange struct {
	Platform string `json:"platform"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// AdDraft is a "Create an ad" draft group.
type AdDraft struct {
	GroupID   string `json:"group_id"`
	Name      string `json:"name"`
	Objective string `json:"objective"`
	// Status is "draft" or "launched" (at least one network started).
	Status string `json:"status"`
	// CanLaunch is true while at least one network never started:
	// Ads.Drafts.Launch can start it.
	CanLaunch bool `json:"can_launch"`
	// Budget is the daily amount per network.
	Budget    AdBudget          `json:"budget"`
	Schedule  AdDraftSchedule   `json:"schedule"`
	Targeting AdDraftTargeting  `json:"targeting"`
	Ads       []AdDraftAd       `json:"ads"`
	Campaigns []AdDraftCampaign `json:"campaigns"`
	// ReviewURL is the builder in the dashboard, where the person checks and
	// starts the ad: hand it over.
	ReviewURL       string            `json:"review_url"`
	CurrencyChanged *AdCurrencyChange `json:"currency_changed,omitempty"`
	UpdatedAt       *string           `json:"updated_at,omitempty"`
}

// AdFollowerShare is the share of the Instagram followers in one country,
// in percent.
type AdFollowerShare struct {
	Country string  `json:"country"`
	Share   float64 `json:"share"`
}

// AdDraftDefaults is the Ads.Drafts.Defaults payload: suggested countries
// and currency for a new ad.
type AdDraftDefaults struct {
	Countries []string `json:"countries"`
	// CountriesSource is "followers", "timezone", "ad_account" or empty.
	CountriesSource string            `json:"countries_source"`
	FollowerShare   []AdFollowerShare `json:"follower_share"`
	Currency        string            `json:"currency"`
	// CurrencySource is "ad_account", "country" or "default".
	CurrencySource string   `json:"currency_source"`
	CreateNetworks []string `json:"create_networks"`
	// Accounts is the workspace's ad account per network.
	Accounts map[string]AdDraftAccount `json:"accounts"`
}

// AdDraftAccountParams is the request body for Ads.Drafts.AttachAccount.
type AdDraftAccountParams struct {
	Platform string `json:"platform"`
	// AdAccountID is from Ads.ListAccounts, the same network.
	AdAccountID AdID `json:"ad_account_id"`
}

// AdDraftLaunchParams is the request body for Ads.Drafts.Launch. Give
// ConfirmBudgetAmount (one daily amount for every network) or
// ConfirmBudgetAmounts (per campaign id). Each must equal that campaign's
// daily amount, else 400 budget_confirmation_required and nothing is
// written. Needs a key with ads:spend: without it the call answers 403
// start_in_app with the draft's page. A network in APIDraftOnly (Google) is
// skipped with code "start_in_app".
type AdDraftLaunchParams struct {
	ConfirmBudgetAmount  float64          `json:"confirm_budget_amount,omitempty"`
	ConfirmBudgetAmounts map[AdID]float64 `json:"confirm_budget_amounts,omitempty"`
}

// AdDraftLaunchItem is the outcome of one network of a launch. The API
// sends CampaignID as a string (64-bit ids); AdID reads it. Code
// "start_in_app" = the person starts that network in the app.
type AdDraftLaunchItem struct {
	CampaignID AdID   `json:"campaign_id"`
	Platform   string `json:"platform"`
	OK         bool   `json:"ok"`
	Code       string `json:"code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// AdDraftLaunchResult is the Ads.Drafts.Launch payload: the draft after the
// launch and one result per network.
type AdDraftLaunchResult struct {
	Group   AdDraft             `json:"group"`
	Results []AdDraftLaunchItem `json:"results"`
}

// ---- Webhook event -----------------------------------------------------------------

// AdStatusChangedEventData is the `data` of an `ad.status_changed` webhook
// delivery: the campaign, not a post. It fires when a campaign changes
// status, for example "pending_review" to "active" (the network's review
// passed and spend started) or to "rejected". Decode the raw delivery into
// a struct with `Data AdStatusChangedEventData` after
// VerifyWebhookSignature accepted it. Subscribing to it needs the ads:read
// scope on the key and access_ads for its person (else 403
// insufficient_scope / insufficient_permission).
type AdStatusChangedEventData struct {
	CampaignID AdID `json:"campaign_id"`
	// WorkspaceID arrives as a number or a string.
	WorkspaceID        json.Number `json:"workspace_id"`
	Name               *string     `json:"name"`
	Platform           *string     `json:"platform"`
	CampaignType       *string     `json:"campaign_type"`
	Status             string      `json:"status"`
	PreviousStatus     *string     `json:"previous_status"`
	ExternalCampaignID *string     `json:"external_campaign_id"`
	Budget             *AdBudget   `json:"budget"`
}

// ---- AdsService methods ----------------------------------------------------------

func adCampaignPath(id AdID) string {
	return "/ads/campaigns/" + url.PathEscape(id.String())
}

// ListAccounts calls `GET /ads/accounts`: the connected ad accounts.
// Accounts are connected and picked in the dashboard; only IsSelected
// accounts can run campaigns. Requires the ads:read scope.
func (s *AdsService) ListAccounts(ctx context.Context) (*AdAccountsResponse, error) {
	var out AdAccountsResponse
	if err := s.client.get(ctx, "/ads/accounts", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Capabilities calls `GET /ads/capabilities`: what each network accepts for
// a new ad (goals, media, text limits, buttons, minimum daily budget),
// CreateNetworks and NetworkStates, and per network the Features its API
// offers, TargetingTypes, BidStrategies, Settings and AdFields, plus
// APIGoals, APIDraftOnly (networks where every API caller stops at the
// draft, also a key with ads:spend: Google) and APIStopInApp (networks where
// the API may not even pause or delete: Google). Requires the ads:read
// scope.
func (s *AdsService) Capabilities(ctx context.Context) (*ItemResponse[AdCapabilities], error) {
	var out ItemResponse[AdCapabilities]
	if err := s.client.get(ctx, "/ads/capabilities", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List calls `GET /ads/campaigns`: campaigns and boosts. A nil params value
// lists them all. Requires the ads:read scope.
func (s *AdsService) List(ctx context.Context, params *AdCampaignListParams) (*ListResponse[AdCampaign], error) {
	query := url.Values{}
	if params != nil {
		if params.Status != "" {
			query.Set("status", params.Status)
		}
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.CampaignType != "" {
			query.Set("campaign_type", params.CampaignType)
		}
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset > 0 {
			query.Set("offset", strconv.Itoa(params.Offset))
		}
	}
	var out ListResponse[AdCampaign]
	if err := s.client.get(ctx, "/ads/campaigns", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /ads/campaigns/:id`: one campaign with its latest numbers.
// Requires the ads:read scope.
func (s *AdsService) Get(ctx context.Context, id AdID) (*ItemResponse[AdCampaign], error) {
	var out ItemResponse[AdCampaign]
	if err := s.client.get(ctx, adCampaignPath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Boost calls `POST /ads/boost`: boost a published post. It creates a DRAFT
// boost that spends nothing; the person starts it in the app. TikTok needs
// Creative.SparkAuthCode, Google needs Headlines, Descriptions and LogoURL.
// Requires the ads:write scope.
func (s *AdsService) Boost(ctx context.Context, params *AdBoostParams) (*ItemResponse[AdCampaign], error) {
	var out ItemResponse[AdCampaign]
	if err := s.client.post(ctx, "/ads/boost", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BoostMulti calls `POST /ads/boost-multi`: one DRAFT boost per ad account
// (up to 10), each with the same budget, so the total is amount x accounts.
// Answers Results; one failed account never stops the others. Requires the
// ads:write scope.
func (s *AdsService) BoostMulti(ctx context.Context, params *AdBoostMultiParams) (*AdMultiResponse[AdMultiBoostResult], error) {
	var out AdMultiResponse[AdMultiBoostResult]
	if err := s.client.post(ctx, "/ads/boost-multi", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /ads/campaigns/:id`: change the name, budget,
// targeting or schedule of a draft. A campaign that reached the network
// changes only with a key that has ads:spend; without it, and on Google
// (APIDraftOnly) always, the call answers 403 start_in_app and a person
// changes it in the app. ConfirmBudgetAmount (the new amount again, after
// the person agreed to it) is for a budget raise of a running campaign. Not
// retried after a 5xx or a connection error: read the campaign with Get to
// see what happened. Requires the ads:write scope.
func (s *AdsService) Update(ctx context.Context, id AdID, params *AdCampaignUpdateParams) (*ItemResponse[AdCampaign], error) {
	var out ItemResponse[AdCampaign]
	if err := s.client.doJSONWith(ctx, http.MethodPatch, adCampaignPath(id), jsonBody(params), &out, retryRateLimit); err != nil {
		return nil, err
	}
	return &out, nil
}

// Activate calls `POST /ads/campaigns/:id/activate`: it STARTS REAL AD
// SPEND. Needs a key with the opt-in ads:spend scope ("Start ads"); without
// it, and on Google (APIDraftOnly) always, the call answers 403
// start_in_app with the page where a person does it in the app. Show the
// person the budget (amount, currency, daily or lifetime) and get their yes
// first; confirmBudgetAmount must equal the campaign's budget amount. A
// campaign of a draft group answers 409 use_draft_launch: use
// Ads.Drafts.Launch. Not retried after a 5xx or a connection error: read
// the campaign with Get to see what happened. Requires the ads:write scope.
func (s *AdsService) Activate(ctx context.Context, id AdID, confirmBudgetAmount float64) (*ItemResponse[AdCampaign], error) {
	body := map[string]any{"confirm_budget_amount": confirmBudgetAmount}
	var out ItemResponse[AdCampaign]
	if err := s.client.doJSONWith(ctx, http.MethodPost, adCampaignPath(id)+"/activate", body, &out, retryRateLimit); err != nil {
		return nil, err
	}
	return &out, nil
}

// ActivateMulti calls `POST /ads/campaigns/activate-multi`: it STARTS REAL
// AD SPEND on up to 10 campaigns. Needs a key with ads:spend: without it,
// and on Google (APIDraftOnly) always, a result has OK false and Code
// "start_in_app" with the page where a person starts it. Every campaign's
// budget must equal confirmBudgetAmount; show the person the total (amount
// x campaigns) and get their yes first. Answers Results. Not retried after
// a 5xx or a connection error: read the campaigns with Get to see what
// happened. Requires the ads:write scope.
func (s *AdsService) ActivateMulti(ctx context.Context, campaignIDs []AdID, confirmBudgetAmount float64) (*AdMultiResponse[AdMultiActivateResult], error) {
	body := map[string]any{"campaign_ids": campaignIDs, "confirm_budget_amount": confirmBudgetAmount}
	var out AdMultiResponse[AdMultiActivateResult]
	if err := s.client.doJSONWith(ctx, http.MethodPost, "/ads/campaigns/activate-multi", body, &out, retryRateLimit); err != nil {
		return nil, err
	}
	return &out, nil
}

// Pause calls `POST /ads/campaigns/:id/pause`: pause a campaign (also while
// in review). Open through the API, except on the networks in APIStopInApp
// (Google): there a campaign that reached the network answers 403
// start_in_app. Requires the ads:write scope.
func (s *AdsService) Pause(ctx context.Context, id AdID) (*ItemResponse[AdCampaign], error) {
	var out ItemResponse[AdCampaign]
	if err := s.client.post(ctx, adCampaignPath(id)+"/pause", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Resume calls `POST /ads/campaigns/:id/resume`: resume a paused campaign,
// which STARTS REAL AD SPEND again. Needs a key with the opt-in ads:spend
// scope ("Start ads"); without it, and on Google (APIDraftOnly) always, the
// call answers 403 start_in_app with the page where a person does it in the
// app. Show the person the budget and get their yes first. Only a paused
// campaign resumes (409 not_paused). Not retried after a 5xx or a
// connection error: read the campaign with Get to see what happened.
// Requires the ads:write scope.
func (s *AdsService) Resume(ctx context.Context, id AdID) (*ItemResponse[AdCampaign], error) {
	var out ItemResponse[AdCampaign]
	if err := s.client.doJSONWith(ctx, http.MethodPost, adCampaignPath(id)+"/resume", nil, &out, retryRateLimit); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /ads/campaigns/:id`: delete a campaign, on the
// network too. A campaign that spent is archived: its spend stays in the
// totals. Open through the API, except a campaign that reached the network
// on a network in APIStopInApp (Google): 403 start_in_app. Requires the
// ads:write scope.
func (s *AdsService) Delete(ctx context.Context, id AdID) (*ItemResponse[AdDeleted], error) {
	var out ItemResponse[AdDeleted]
	if err := s.client.do(ctx, http.MethodDelete, adCampaignPath(id), nil, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Overview calls `GET /ads/overview`: accounts, campaigns per status and
// the totals. Totals.Spend is nil when the campaigns use more than one
// currency: read TotalsByCurrency then. Requires the ads:read scope.
func (s *AdsService) Overview(ctx context.Context) (*ItemResponse[AdsOverview], error) {
	var out ItemResponse[AdsOverview]
	if err := s.client.get(ctx, "/ads/overview", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Suggestions calls `GET /ads/suggestions`: organic posts that do better
// than the workspace median, boost candidates. Read-only. Requires the
// ads:read scope.
func (s *AdsService) Suggestions(ctx context.Context) (*ListResponse[BoostSuggestion], error) {
	var out ListResponse[BoostSuggestion]
	if err := s.client.get(ctx, "/ads/suggestions", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResearchKeywords calls `POST /ads/research/keywords`: keyword ideas from
// seed keywords or a URL, through the connected Google Ads account.
// Read-only and free. Requires the ads:read scope.
func (s *AdsService) ResearchKeywords(ctx context.Context, params *AdKeywordResearchParams) (*ItemResponse[AdKeywordResearchResult], error) {
	var out ItemResponse[AdKeywordResearchResult]
	if err := s.client.post(ctx, "/ads/research/keywords", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// KeywordMetrics calls `POST /ads/research/keywords/metrics`: search
// numbers for an exact keyword list. Read-only. Requires the ads:read scope.
func (s *AdsService) KeywordMetrics(ctx context.Context, params *AdKeywordMetricsParams) (*ItemResponse[AdKeywordMetricsResult], error) {
	var out ItemResponse[AdKeywordMetricsResult]
	if err := s.client.post(ctx, "/ads/research/keywords/metrics", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- AdDraftsService methods -----------------------------------------------------

func adDraftPath(groupID string) string {
	return "/ads/drafts/" + url.PathEscape(groupID)
}

// Defaults calls `GET /ads/drafts/defaults`: suggested countries (where the
// Instagram followers are), the currency and the workspace's ad account per
// network. Requires the ads:read scope.
func (s *AdDraftsService) Defaults(ctx context.Context) (*ItemResponse[AdDraftDefaults], error) {
	var out ItemResponse[AdDraftDefaults]
	if err := s.client.get(ctx, "/ads/drafts/defaults", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /ads/drafts`: save a new ad as a draft group. It needs
// no ad account and spends nothing. Read Ads.Capabilities first for what
// each network accepts. The answer carries ReviewURL (the builder in the
// dashboard, where the person checks and starts the ad: hand it over) and
// per network Readiness. A Budget.Currency other than the attached ad
// account's currency answers 400 currency_mismatch with param
// "budget.currency". Requires the ads:write scope.
func (s *AdDraftsService) Create(ctx context.Context, params *AdDraftParams) (*ItemResponse[AdDraft], error) {
	var out ItemResponse[AdDraft]
	if err := s.client.post(ctx, "/ads/drafts", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /ads/drafts/:group_id`: one draft group. Requires the
// ads:read scope.
func (s *AdDraftsService) Get(ctx context.Context, groupID string) (*ItemResponse[AdDraft], error) {
	var out ItemResponse[AdDraft]
	if err := s.client.get(ctx, adDraftPath(groupID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PUT /ads/drafts/:group_id`: replace the draft with the full
// body. Networks that already started are left alone
// (AdDraftCampaign.Editable). A Budget.Currency other than the attached ad
// account's currency answers 400 currency_mismatch with param
// "budget.currency". Requires the ads:write scope.
func (s *AdDraftsService) Update(ctx context.Context, groupID string, params *AdDraftParams) (*ItemResponse[AdDraft], error) {
	var out ItemResponse[AdDraft]
	if err := s.client.put(ctx, adDraftPath(groupID), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /ads/drafts/:group_id`: delete a draft group that
// never started. Requires the ads:write scope.
func (s *AdDraftsService) Delete(ctx context.Context, groupID string) (*ItemResponse[AdDeleted], error) {
	var out ItemResponse[AdDeleted]
	if err := s.client.do(ctx, http.MethodDelete, adDraftPath(groupID), nil, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AttachAccount calls `POST /ads/drafts/:group_id/account`: set the ad
// account of one network in the draft (from Ads.ListAccounts, the same
// network). Requires the ads:write scope.
func (s *AdDraftsService) AttachAccount(ctx context.Context, groupID string, params *AdDraftAccountParams) (*ItemResponse[AdDraft], error) {
	var out ItemResponse[AdDraft]
	if err := s.client.post(ctx, adDraftPath(groupID)+"/account", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Launch calls `POST /ads/drafts/:group_id/launch`: it STARTS REAL AD SPEND
// for every network of the draft that never started. Needs a key with the
// opt-in ads:spend scope; without it the call answers 403 start_in_app with
// the page where a person starts it (the draft's ReviewURL), and a network
// in APIDraftOnly (Google) is always skipped with code "start_in_app". Show
// the person the daily amount per network and get their yes first, then
// pass it as ConfirmBudgetAmount (or per campaign as ConfirmBudgetAmounts).
// A wrong or missing amount answers 400 budget_confirmation_required and
// writes nothing. Results per network (CampaignID arrives as a string); one
// failed network never stops the others. Not retried after a 5xx or a
// connection error: read the draft with Get to see what started. Requires
// the ads:write scope.
func (s *AdDraftsService) Launch(ctx context.Context, groupID string, params *AdDraftLaunchParams) (*ItemResponse[AdDraftLaunchResult], error) {
	var out ItemResponse[AdDraftLaunchResult]
	if err := s.client.doJSONWith(ctx, http.MethodPost, adDraftPath(groupID)+"/launch", jsonBody(params), &out, retryRateLimit); err != nil {
		return nil, err
	}
	return &out, nil
}
