package omnisocials

// The Ads API parity endpoints (2026-11-05): the structure of an ad account
// (also the campaigns made outside OmniSocials), the ads of a campaign,
// targeting search, custom audiences, lead forms and leads, conversions,
// insights and background reports, automated rules, your own results, the
// network's media library, previews, partnership ads, comments on ads, ad
// account admin, Meta reach and frequency, studies, budget schedules and
// Google reports.
//
// The SDK is a thin passthrough: the API checks every field. A network that
// has no such feature answers 400 not_supported_on_network
// (AdCapabilities.Features lists the features per network). A missing
// permission on the network side answers 403 network_permission_missing.
//
// Money moments (switching something on, a budget or spend cap change, a
// rule that can raise spend, a reach and frequency reservation, a study, a
// planned budget increase) and other changes of a campaign on the network
// need a key with the opt-in ads:spend scope ("Start ads"): without it, and
// on Google always, they answer 403 start_in_app with the page where a
// person does it in the app (rules: 403 rule_raises_spend). Pausing,
// deleting and archiving stay open, except on Google.
//
// Money rule: every call that can start or raise spend (Objects.Update,
// Objects.SetStatus, Ads.BulkStatus, CampaignAds.SetStatus, Rules.Create,
// Rules.Update, Rules.Undo, Accounts.UpdateSettings, ReachFrequency.Reserve,
// BudgetSchedules.Create) takes the person's own amount
// (ConfirmBudgetAmount, for rules ConfirmMaxDailyBudget; with ads:spend a
// wrong or missing one answers 400 budget_confirmation_required) and is
// retried after a 429 only, never after a 5xx or a connection error: the
// first attempt may have gone through. Never invent or confirm an amount
// yourself.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// AdsWebhookEvents are the ads webhook events. ad.status_changed carries the
// campaign (AdStatusChangedEventData); the campaign.* and ad.* events carry
// the campaign plus "details"; lead.received, ads.* and ad_rule.triggered
// carry their own data (a lead, an ad account, a rule run).
var AdsWebhookEvents = []string{
	"ad.status_changed",
	"ad.created",
	"ad.updated",
	"ad.review",
	"ad.delivery_issue",
	"campaign.created",
	"campaign.updated",
	"campaign.budget",
	"lead.received",
	"ads.sync_failed",
	"ads.sync_recovered",
	"ads.initial_sync_completed",
	"ad_rule.triggered",
}

// ---- Services ------------------------------------------------------------------

// AdObjectsService covers the campaigns, ad sets and ads of an ad account as
// the network has them (also the ones made outside OmniSocials) and the
// import of those campaigns. Reach it through client.Ads.Objects. Objects of
// OmniSocials campaigns answer 409 use_campaign_routes: change those with the
// campaign methods.
type AdObjectsService struct{ client *Client }

// AdCampaignAdsService covers the ads of an OmniSocials campaign.
type AdCampaignAdsService struct{ client *Client }

// AdCommentsService covers the comments on an ad.
type AdCommentsService struct{ client *Client }

// AdTargetingService covers targeting search, browse, reach estimates and
// bid suggestions.
type AdTargetingService struct{ client *Client }

// AdAudiencesService covers custom audiences: customer lists, lookalikes and
// company lists.
type AdAudiencesService struct{ client *Client }

// AdLeadFormsService covers lead forms.
type AdLeadFormsService struct{ client *Client }

// AdLeadsService covers the leads copied from the lead forms.
type AdLeadsService struct{ client *Client }

// AdConversionsService covers pixels, conversion goals, custom conversions
// and server-side conversion events.
type AdConversionsService struct{ client *Client }

// AdInsightsService covers live numbers, campaign and ad analytics, the
// timeline, rankings and alerts.
type AdInsightsService struct{ client *Client }

// AdReportsService covers long reports that run in the background.
type AdReportsService struct{ client *Client }

// AdRulesService covers automated rules: check campaigns on a schedule and
// notify, pause, activate or adjust the budget. A rule that notifies,
// pauses or lowers a budget works with ads:write. A rule that can raise
// spend (activate, a positive adjust_budget) needs a key with ads:spend
// (else 403 rule_raises_spend) and Action.MaxDailyBudget, confirmed by the
// person as ConfirmMaxDailyBudget; it never acts on Google.
type AdRulesService struct{ client *Client }

// AdResultsService covers your own numbers per campaign and day.
type AdResultsService struct{ client *Client }

// AdMediaService covers the network's own image and video library.
type AdMediaService struct{ client *Client }

// AdPreviewsService covers previews drawn by the network.
type AdPreviewsService struct{ client *Client }

// AdPartnershipService covers partnership ads with creators (Meta).
type AdPartnershipService struct{ client *Client }

// AdAccountsService covers ad account admin: identities, health, finance,
// settings, people, the change log, businesses, DSA and the manager
// hierarchy. List the ad accounts with Ads.ListAccounts.
type AdAccountsService struct{ client *Client }

// AdReachFrequencyService covers Meta reach and frequency buying.
type AdReachFrequencyService struct{ client *Client }

// AdStudiesService covers A/B tests and lift studies (Meta).
type AdStudiesService struct{ client *Client }

// AdBudgetSchedulesService covers planned budget increases (Meta).
type AdBudgetSchedulesService struct{ client *Client }

// AdGoogleService covers the Google reports (read-only; every API caller
// stops at the draft on Google).
type AdGoogleService struct{ client *Client }

// newAdsService wires client.Ads and its sub-services.
func newAdsService(c *Client) *AdsService {
	return &AdsService{
		client:          c,
		Drafts:          &AdDraftsService{client: c},
		Objects:         &AdObjectsService{client: c},
		CampaignAds:     &AdCampaignAdsService{client: c},
		Comments:        &AdCommentsService{client: c},
		Targeting:       &AdTargetingService{client: c},
		Audiences:       &AdAudiencesService{client: c},
		LeadForms:       &AdLeadFormsService{client: c},
		Leads:           &AdLeadsService{client: c},
		Conversions:     &AdConversionsService{client: c},
		Insights:        &AdInsightsService{client: c},
		Reports:         &AdReportsService{client: c},
		Rules:           &AdRulesService{client: c},
		Results:         &AdResultsService{client: c},
		Media:           &AdMediaService{client: c},
		Previews:        &AdPreviewsService{client: c},
		Partnership:     &AdPartnershipService{client: c},
		Accounts:        &AdAccountsService{client: c},
		ReachFrequency:  &AdReachFrequencyService{client: c},
		Studies:         &AdStudiesService{client: c},
		BudgetSchedules: &AdBudgetSchedulesService{client: c},
		Google:          &AdGoogleService{client: c},
	}
}

// ---- Shared shapes --------------------------------------------------------------

// AdsData is a `data` payload the API documents loosely.
type AdsData map[string]any

// AdMetrics are numbers in major currency units; nil means the network does
// not report it. CTR is a percent.
type AdMetrics struct {
	Spend           *float64 `json:"spend,omitempty"`
	Impressions     *float64 `json:"impressions,omitempty"`
	Clicks          *float64 `json:"clicks,omitempty"`
	CTR             *float64 `json:"ctr,omitempty"`
	CPC             *float64 `json:"cpc,omitempty"`
	CPM             *float64 `json:"cpm,omitempty"`
	Reach           *float64 `json:"reach,omitempty"`
	Frequency       *float64 `json:"frequency,omitempty"`
	Conversions     *float64 `json:"conversions,omitempty"`
	ConversionValue *float64 `json:"conversion_value,omitempty"`
	VideoViews      *float64 `json:"video_views,omitempty"`
	Leads           *float64 `json:"leads,omitempty"`
	Engagements     *float64 `json:"engagements,omitempty"`
}

// AdTargetingItem is one targeting entry: an id from Targeting.Search of the
// same network.
type AdTargetingItem struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// RadiusKm is 1 to 80, only on include "location".
	RadiusKm float64 `json:"radius_km,omitempty"`
}

// AdAudienceRef is a custom audience of the same ad account.
type AdAudienceRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// AdAudienceRefs are the custom audiences to include and exclude.
type AdAudienceRefs struct {
	Include []AdAudienceRef `json:"include,omitempty"`
	Exclude []AdAudienceRef `json:"exclude,omitempty"`
}

// AdNetworkTargeting is the targeting of one network:
// AdDraftTargeting.Networks[network] on a draft, "network" in a campaign's
// targeting. The keys of Include and Exclude are the network's
// AdCapabilities.TargetingTypes; Include["location"] replaces the countries
// on that network.
type AdNetworkTargeting struct {
	Include   map[string][]AdTargetingItem `json:"include,omitempty"`
	Exclude   map[string][]AdTargetingItem `json:"exclude,omitempty"`
	Audiences *AdAudienceRefs              `json:"audiences,omitempty"`
}

// AdConversionSetting is the pixel or tag and the event a conversions
// campaign optimises for.
type AdConversionSetting struct {
	DestinationID      string `json:"destination_id"`
	Event              string `json:"event,omitempty"`
	CustomConversionID string `json:"custom_conversion_id,omitempty"`
}

// AdBiddingSetting is the bid strategy: "lowest_cost", "cost_cap",
// "bid_cap", "min_roas" or "target_cost".
type AdBiddingSetting struct {
	Strategy  string  `json:"strategy"`
	Amount    float64 `json:"amount,omitempty"`
	ROASFloor float64 `json:"roas_floor,omitempty"`
}

// AdAttributionSetting is the attribution window (ClickDays 1, 7 or 28,
// ViewDays 0 or 1).
type AdAttributionSetting struct {
	ClickDays int  `json:"click_days,omitempty"`
	ViewDays  *int `json:"view_days,omitempty"`
}

// AdOptimizationSettings groups conversion, bidding and attribution.
type AdOptimizationSettings struct {
	Conversion  *AdConversionSetting  `json:"conversion,omitempty"`
	Bidding     *AdBiddingSetting     `json:"bidding,omitempty"`
	Attribution *AdAttributionSetting `json:"attribution,omitempty"`
}

// AdWelcomeMessage is the first message of a Meta messages ad.
type AdWelcomeMessage struct {
	Text         string   `json:"text"`
	QuickReplies []string `json:"quick_replies,omitempty"`
}

// AdMessagingSettings is Meta's goal messages: Destination is "messenger",
// "instagram_direct" or "whatsapp".
type AdMessagingSettings struct {
	Destination         string            `json:"destination"`
	WhatsAppPhoneNumber string            `json:"whatsapp_phone_number,omitempty"`
	WelcomeMessage      *AdWelcomeMessage `json:"welcome_message,omitempty"`
	// AutomationID is a DM automation that answers the first message.
	AutomationID int64 `json:"automation_id,omitempty"`
}

// AdCallsSettings is Meta's goal calls: an E.164 number.
type AdCallsSettings struct {
	PhoneNumber string `json:"phone_number"`
}

// AdGoogleSettings is the Google campaign kind: "demand_gen", "search" or
// "performance_max", with its own fields.
type AdGoogleSettings struct {
	Kind           string         `json:"kind,omitempty"`
	Search         map[string]any `json:"search,omitempty"`
	PerformanceMax map[string]any `json:"performance_max,omitempty"`
}

// AdMetaSettings: BudgetLevel is "ad_set" (default) or "campaign".
type AdMetaSettings struct {
	BudgetLevel string `json:"budget_level,omitempty"`
}

// AdNetworkSettings are the settings of one network: AdDraftPlatform.Settings
// on a draft, Settings on Ads.Update (while the campaign is a draft).
// AdNetworkCapabilities.Settings lists the keys each network reads.
type AdNetworkSettings struct {
	Optimization *AdOptimizationSettings `json:"optimization,omitempty"`
	Messaging    *AdMessagingSettings    `json:"messaging,omitempty"`
	Calls        *AdCallsSettings        `json:"calls,omitempty"`
	// LeadFormID is the network's lead form id (AdLeadForm.ExternalID),
	// goal lead_generation.
	LeadFormID string            `json:"lead_form_id,omitempty"`
	Google     *AdGoogleSettings `json:"google,omitempty"`
	Meta       *AdMetaSettings   `json:"meta,omitempty"`
}

// AdMediaRef points at a file in the ad account's library (Ads.Media): Meta
// ImageHash or VideoID, TikTok ImageID or VideoID, X MediaKey.
type AdMediaRef struct {
	ImageHash string `json:"image_hash,omitempty"`
	ImageID   string `json:"image_id,omitempty"`
	VideoID   string `json:"video_id,omitempty"`
	MediaKey  string `json:"media_key,omitempty"`
}

// AdMetaAdFields are the Meta extras of one ad (AdDraftAd.Network["meta"]).
type AdMetaAdFields struct {
	LeadFormID string `json:"lead_form_id,omitempty"`
	// PrimaryTexts and Headlines: up to 4 more next to the ad's own
	// (website goals only).
	PrimaryTexts     []string          `json:"primary_texts,omitempty"`
	Headlines        []string          `json:"headlines,omitempty"`
	CreativeFeatures map[string]string `json:"creative_features,omitempty"`
	MediaRef         *AdMediaRef       `json:"media_ref,omitempty"`
	// Partnership is a partnership ad: the partner's Instagram account id.
	Partnership *struct {
		SponsorID string `json:"sponsor_id"`
	} `json:"partnership,omitempty"`
}

// AdTikTokAdFields are the TikTok extras of one ad.
type AdTikTokAdFields struct {
	SparkAuthCode string      `json:"spark_auth_code,omitempty"`
	LeadFormID    string      `json:"lead_form_id,omitempty"`
	MediaRef      *AdMediaRef `json:"media_ref,omitempty"`
}

// AdResponsiveSearchAd is a Google Search ad (AdDraftAd.Network["google"]
// key "responsive_search_ad"): 3 to 15 headlines, 2 to 4 descriptions.
type AdResponsiveSearchAd struct {
	Headlines    []string `json:"headlines,omitempty"`
	Descriptions []string `json:"descriptions,omitempty"`
	Path1        string   `json:"path1,omitempty"`
	Path2        string   `json:"path2,omitempty"`
	FinalURL     string   `json:"final_url,omitempty"`
}

// AdCampaignTargeting is a campaign's targeting with the network's own part.
// Ads.Update takes targeting as a map; this type documents the keys.
type AdCampaignTargeting struct {
	Countries []string            `json:"countries,omitempty"`
	AgeMin    int                 `json:"age_min,omitempty"`
	AgeMax    int                 `json:"age_max,omitempty"`
	Genders   []string            `json:"genders,omitempty"`
	Network   *AdNetworkTargeting `json:"network,omitempty"`
}

// AdStatusParams pauses ("paused") or switches on ("active") an object or
// an ad. Switching on needs ConfirmBudgetAmount = the budget that pays.
type AdStatusParams struct {
	Status              string  `json:"status"`
	ConfirmBudgetAmount float64 `json:"confirm_budget_amount,omitempty"`
}

// AdDateRange is an ISO date range (YYYY-MM-DD).
type AdDateRange struct {
	From string `url:"from"`
	To   string `url:"to"`
}

// ---- Structure ----------------------------------------------------------------------

// AdTreeNode is one campaign, ad set or ad as the network has it.
type AdTreeNode struct {
	// Level is "campaign", "ad_set" or "ad".
	Level      string     `json:"level"`
	ExternalID string     `json:"external_id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Budget     *AdBudget  `json:"budget"`
	Objective  *string    `json:"objective"`
	StartAt    *string    `json:"start_at"`
	EndAt      *string    `json:"end_at"`
	Metrics    *AdMetrics `json:"metrics"`
	// Review and Creative are set on ads.
	Review   *AdReview      `json:"review,omitempty"`
	Creative map[string]any `json:"creative,omitempty"`
	// Source is "omnisocials" or "external" (made outside OmniSocials).
	Source     string       `json:"source,omitempty"`
	CampaignID *AdID        `json:"campaign_id,omitempty"`
	Children   []AdTreeNode `json:"children,omitempty"`
}

// AdTree is the Objects.Tree payload.
type AdTree struct {
	AdAccountID AdID         `json:"ad_account_id"`
	Platform    string       `json:"platform"`
	Currency    *string      `json:"currency"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	Campaigns   []AdTreeNode `json:"campaigns"`
	NextCursor  *string      `json:"next_cursor"`
}

// AdTreeParams filters Objects.Tree. Status is a list (sent as a comma list).
type AdTreeParams struct {
	From   string   `url:"from"`
	To     string   `url:"to"`
	Status []string `url:"status"`
	// Limit is at most 100 (default 25).
	Limit  int    `url:"limit"`
	Cursor string `url:"cursor"`
}

// AdObjectUpdateParams is the body of Objects.Update. A higher budget needs
// ConfirmBudgetAmount = the new amount. EndAt: a date-time string, or
// omnisocials.Null to remove the end date.
type AdObjectUpdateParams struct {
	Name                string    `json:"name,omitempty"`
	Budget              *AdBudget `json:"budget,omitempty"`
	EndAt               any       `json:"end_at,omitempty"`
	ConfirmBudgetAmount float64   `json:"confirm_budget_amount,omitempty"`
}

// AdObjectStatusResult is the Objects.SetStatus payload.
type AdObjectStatusResult struct {
	Level      string    `json:"level"`
	ExternalID string    `json:"external_id"`
	Status     string    `json:"status"`
	Budget     *AdBudget `json:"budget,omitempty"`
}

// AdObjectDuplicateParams is the body of Objects.Duplicate. Deep (default
// true) copies the children too.
type AdObjectDuplicateParams struct {
	Deep   *bool  `json:"deep,omitempty"`
	Rename string `json:"rename,omitempty"`
}

// AdImportState is the import state of an ad account: Status is
// "not_started", "pending", "queued", "done" or "failed".
type AdImportState struct {
	AdAccountID       AdID    `json:"ad_account_id"`
	Platform          string  `json:"platform"`
	Status            string  `json:"status"`
	CampaignsImported int     `json:"campaigns_imported,omitempty"`
	FirstImportAt     *string `json:"first_import_at,omitempty"`
	LastRunAt         *string `json:"last_run_at,omitempty"`
}

// AdExternalCampaignListParams filters Objects.ListExternal.
type AdExternalCampaignListParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Platform    string `url:"platform"`
	Status      string `url:"status"`
	// Limit is at most 200.
	Limit  int `url:"limit"`
	Offset int `url:"offset"`
}

// AdExternalCampaigns is the Objects.ListExternal payload.
type AdExternalCampaigns struct {
	Campaigns []AdsData `json:"campaigns"`
	Total     int       `json:"total"`
}

// ---- Campaign level -----------------------------------------------------------------

// AdBulkStatusParams is the body of Ads.BulkStatus. Resume ("active") needs
// ConfirmBudgetAmount = every campaign's budget.
type AdBulkStatusParams struct {
	// CampaignIDs: at most 50.
	CampaignIDs         []AdID  `json:"campaign_ids"`
	Status              string  `json:"status"`
	ConfirmBudgetAmount float64 `json:"confirm_budget_amount,omitempty"`
}

// AdBulkStatusResult is one campaign of Ads.BulkStatus.
type AdBulkStatusResult struct {
	CampaignID AdID   `json:"campaign_id"`
	OK         bool   `json:"ok"`
	Status     string `json:"status,omitempty"`
	Code       string `json:"code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// AdBulkStatusResults is the Ads.BulkStatus payload.
type AdBulkStatusResults struct {
	Results []AdBulkStatusResult `json:"results"`
}

// AdReview is a network's review verdict: Status "in_review", "approved" or
// "rejected", with Reasons.
type AdReview struct {
	Status  *string  `json:"status"`
	Reasons []string `json:"reasons"`
}

// AdCampaignAdCreative is the creative of an ad of an OmniSocials campaign.
type AdCampaignAdCreative struct {
	Headline    *string `json:"headline"`
	PrimaryText *string `json:"primary_text"`
	Link        *string `json:"link"`
	Media       []any   `json:"media"`
	Variant     *string `json:"variant"`
}

// AdCampaignAd is one ad of an OmniSocials campaign.
type AdCampaignAd struct {
	ID           AdID                  `json:"id"`
	CampaignID   *AdID                 `json:"campaign_id,omitempty"`
	Kind         string                `json:"kind"`
	Name         *string               `json:"name"`
	Position     *int                  `json:"position"`
	PostID       *AdID                 `json:"post_id"`
	ExternalAdID *string               `json:"external_ad_id"`
	Status       string                `json:"status"`
	Review       AdReview              `json:"review"`
	Errors       *string               `json:"errors"`
	Metrics      map[string]any        `json:"metrics"`
	Creative     *AdCampaignAdCreative `json:"creative"`
	CreatedAt    *string               `json:"created_at"`
}

// AdCampaignAds is the CampaignAds.List and CampaignAds.Add payload.
type AdCampaignAds struct {
	CampaignID AdID           `json:"campaign_id"`
	Platform   string         `json:"platform,omitempty"`
	Ads        []AdCampaignAd `json:"ads"`
}

// ---- Targeting ----------------------------------------------------------------------

// AdTargetingSearchParams is the query of Targeting.Search. Type is one of
// the network's AdCapabilities.TargetingTypes.
type AdTargetingSearchParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Type        string `url:"type"`
	Q           string `url:"q"`
	// Limit is at most 100.
	Limit   int    `url:"limit"`
	Country string `url:"country"`
}

// AdTargetingBrowseParams is the query of Targeting.Browse.
type AdTargetingBrowseParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Type        string `url:"type"`
}

// AdTargetingOption is one search or browse result.
type AdTargetingOption struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Type              string              `json:"type,omitempty"`
	Path              any                 `json:"path,omitempty"`
	AudienceSizeLower *float64            `json:"audience_size_lower,omitempty"`
	AudienceSizeUpper *float64            `json:"audience_size_upper,omitempty"`
	Children          []AdTargetingOption `json:"children,omitempty"`
}

// AdTargetingResults is the Targeting.Search and Targeting.Browse payload.
type AdTargetingResults struct {
	Platform string              `json:"platform"`
	Type     string              `json:"type"`
	Items    []AdTargetingOption `json:"items"`
}

// AdReachEstimateParams is the body of Targeting.ReachEstimate and
// Targeting.BidSuggestions (Budget is for bid suggestions only).
type AdReachEstimateParams struct {
	AdAccountID AdID                `json:"ad_account_id"`
	Objective   string              `json:"objective,omitempty"`
	Targeting   AdCampaignTargeting `json:"targeting"`
	Budget      *AdBudget           `json:"budget,omitempty"`
}

// ---- Audiences ----------------------------------------------------------------------

// AdAudienceListParams is the query of Audiences.List.
type AdAudienceListParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Cursor      string `url:"cursor"`
}

// AdLookalike is the source, country and ratio (0.01 to 0.2) of a
// lookalike audience.
type AdLookalike struct {
	SourceAudienceID string  `json:"source_audience_id"`
	Country          string  `json:"country"`
	Ratio            float64 `json:"ratio"`
}

// AdAudienceCreateParams is the body of Audiences.Create. Kind is
// "customer_list", "lookalike" or "company_list". CustomerFileSource (Meta:
// "user_provided", "partner_provided" or "both") is what the person says.
// Schema + Rows are optional first members of a customer list (Pinterest
// needs them); raw values are hashed with SHA-256 before they leave.
type AdAudienceCreateParams struct {
	AdAccountID        AdID         `json:"ad_account_id"`
	Name               string       `json:"name"`
	Description        string       `json:"description,omitempty"`
	Kind               string       `json:"kind,omitempty"`
	Lookalike          *AdLookalike `json:"lookalike,omitempty"`
	CustomerFileSource string       `json:"customer_file_source,omitempty"`
	Schema             []string     `json:"schema,omitempty"`
	Rows               [][]string   `json:"rows,omitempty"`
}

// AdAudienceUpdateParams is the body of Audiences.Update.
type AdAudienceUpdateParams struct {
	AdAccountID AdID   `json:"ad_account_id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// AdAudienceMembersParams is the body of Audiences.AddMembers and
// Audiences.RemoveMembers. Send the raw values: OmniSocials normalizes them
// and hashes them with SHA-256 before they leave and stores nothing. At
// most 10,000 rows. Schema keys: email, phone (with the country code),
// first_name, last_name, city, state, zip, country, external_id, dob_year,
// gender, maid.
type AdAudienceMembersParams struct {
	AdAccountID AdID       `json:"ad_account_id"`
	Schema      []string   `json:"schema"`
	Rows        [][]string `json:"rows"`
}

// AdAudienceCompany is one company of a company list.
type AdAudienceCompany struct {
	Name        string `json:"name,omitempty"`
	Domain      string `json:"domain,omitempty"`
	LinkedInURL string `json:"linkedin_url,omitempty"`
}

// AdAudienceCompaniesParams is the body of Audiences.AddCompanies.
type AdAudienceCompaniesParams struct {
	AdAccountID AdID                `json:"ad_account_id"`
	Companies   []AdAudienceCompany `json:"companies"`
}

// ---- Lead forms and leads ---------------------------------------------------------------

// AdLeadFormQuestion is one question: Type "email", "full_name",
// "first_name", "last_name", "phone", "company", "job_title", "city",
// "country", "zip" or "custom".
type AdLeadFormQuestion struct {
	Type    string   `json:"type"`
	Label   string   `json:"label,omitempty"`
	Options []string `json:"options,omitempty"`
}

// AdLeadFormThankYou is the screen after the form.
type AdLeadFormThankYou struct {
	Title      string `json:"title,omitempty"`
	Body       string `json:"body,omitempty"`
	ButtonText string `json:"button_text,omitempty"`
	URL        string `json:"url,omitempty"`
}

// AdLeadFormCreateParams is the body of LeadForms.Create. AcceptTerms says
// the person accepts the network's lead ad terms (Pinterest refuses a form
// without it): only set it after the person said yes.
type AdLeadFormCreateParams struct {
	AdAccountID      AdID                 `json:"ad_account_id"`
	Name             string               `json:"name"`
	PrivacyPolicyURL string               `json:"privacy_policy_url"`
	Locale           string               `json:"locale,omitempty"`
	Headline         string               `json:"headline,omitempty"`
	Description      string               `json:"description,omitempty"`
	Questions        []AdLeadFormQuestion `json:"questions"`
	ThankYou         AdLeadFormThankYou   `json:"thank_you"`
	AcceptTerms      *bool                `json:"accept_terms,omitempty"`
}

// AdLeadFormUpdateParams is the body of LeadForms.Update: Status "active"
// or "archived", or the lead sync on or off.
type AdLeadFormUpdateParams struct {
	Status      string `json:"status,omitempty"`
	SyncEnabled *bool  `json:"sync_enabled,omitempty"`
}

// AdLeadListParams filters Leads.List.
type AdLeadListParams struct {
	FormID AdID `url:"form_id"`
	// Status is "new", "contacted", "qualified", "won", "lost" or "spam".
	Status string `url:"status"`
	Since  string `url:"since"`
	Until  string `url:"until"`
	// Limit is at most 200.
	Limit  int    `url:"limit"`
	Cursor string `url:"cursor"`
}

// AdLeadUpdateParams is the body of Leads.Update.
type AdLeadUpdateParams struct {
	Status string `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

// AdLeadCounts is the Leads.Counts payload.
type AdLeadCounts struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// ---- Conversions --------------------------------------------------------------------

// AdCustomConversionParams is the body of Conversions.CreateCustom.
type AdCustomConversionParams struct {
	AdAccountID   AdID   `json:"ad_account_id"`
	Name          string `json:"name"`
	DestinationID string `json:"destination_id"`
	Event         string `json:"event,omitempty"`
	URLContains   string `json:"url_contains,omitempty"`
}

// AdConversionEventsParams is the body of Conversions.SendEvents: at most
// 1,000 events. Each event is a map with event_name (required), event_time
// (ISO date-time or unix seconds), event_id, action_source,
// event_source_url, value, currency, content_ids, order_id and user (email,
// phone, external_id, client_ip, client_user_agent, click_ids). Send the raw
// email and phone: OmniSocials normalizes and SHA-256 hashes them before
// they leave and stores nothing.
type AdConversionEventsParams struct {
	AdAccountID   AdID             `json:"ad_account_id"`
	DestinationID string           `json:"destination_id"`
	Events        []map[string]any `json:"events"`
}

// ---- Insights and reports -------------------------------------------------------------

// AdInsightsQueryParams is the body of Insights.Query and Reports.Start.
type AdInsightsQueryParams struct {
	AdAccountID AdID `json:"ad_account_id"`
	// Level is "account", "campaign" (default), "ad_set" or "ad".
	Level string   `json:"level,omitempty"`
	IDs   []string `json:"ids,omitempty"`
	// Metrics: spend, impressions, clicks, ctr, cpc, cpm, reach, frequency,
	// conversions, conversion_value, video_views, leads, engagements.
	Metrics []string `json:"metrics,omitempty"`
	// Breakdowns: at most 2 of country, age, gender, placement, device,
	// platform.
	Breakdowns []string `json:"breakdowns,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	// TimeIncrement is "day" or "all".
	TimeIncrement string `json:"time_increment,omitempty"`
}

// AdInsightsRow is one row of numbers.
type AdInsightsRow struct {
	Level      string            `json:"level"`
	ExternalID *string           `json:"external_id"`
	Date       *string           `json:"date"`
	Breakdown  map[string]string `json:"breakdown"`
	Metrics    AdMetrics         `json:"metrics"`
}

// AdInsightsRows is the payload of the insights reads.
type AdInsightsRows struct {
	Rows []AdInsightsRow `json:"rows"`
}

// AdCampaignAnalyticsParams is the query of Insights.Campaign and
// Insights.Ad. Breakdown is "ad" (campaign only), "country", "age",
// "gender", "placement", "device" or "platform".
type AdCampaignAnalyticsParams struct {
	From          string   `url:"from"`
	To            string   `url:"to"`
	Breakdown     string   `url:"breakdown"`
	TimeIncrement string   `url:"time_increment"`
	Metrics       []string `url:"metrics"`
}

// AdTimelineParams is the query of Insights.Timeline. Source is "all",
// "omnisocials" or "external".
type AdTimelineParams struct {
	From     string `url:"from"`
	To       string `url:"to"`
	Platform string `url:"platform"`
	Source   string `url:"source"`
}

// AdPerformanceParams is the query of Insights.Performance.
type AdPerformanceParams struct {
	// Level is "campaign" or "ad".
	Level          string `url:"level"`
	Metric         string `url:"metric"`
	Order          string `url:"order"`
	Limit          int    `url:"limit"`
	MinImpressions int    `url:"min_impressions"`
	From           string `url:"from"`
	To             string `url:"to"`
	Platform       string `url:"platform"`
}

// AdReport is a background report: Status "running", "done" (with Rows)
// or "failed".
type AdReport struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Rows   []AdInsightsRow `json:"rows,omitempty"`
}

// ---- Rules -------------------------------------------------------------------------

// AdRuleScope says which campaigns a rule checks.
type AdRuleScope struct {
	// Source is "omnisocials", "external" or "all".
	Source              string   `json:"source,omitempty"`
	Platforms           []string `json:"platforms,omitempty"`
	CampaignIDs         []AdID   `json:"campaign_ids,omitempty"`
	ExternalCampaignIDs []string `json:"external_campaign_ids,omitempty"`
	Statuses            []string `json:"statuses,omitempty"`
}

// AdRuleCondition is one condition: Metric (spend, impressions, clicks, ctr,
// cpc, cpm, conversions, leads, reach, cost_per_conversion, cost_per_lead,
// results.<key>, formula.<key>), Op ("gt", "gte", "lt", "lte"), Value and
// WindowDays (1 to 30).
type AdRuleCondition struct {
	Metric     string  `json:"metric"`
	Op         string  `json:"op"`
	Value      float64 `json:"value"`
	WindowDays int     `json:"window_days,omitempty"`
}

// AdRuleAction is what a rule does: Type "notify", "pause", "activate" or
// "adjust_budget" (Percent). A rule that can raise spend needs
// MaxDailyBudget.
type AdRuleAction struct {
	Type           string  `json:"type"`
	Percent        float64 `json:"percent,omitempty"`
	MaxDailyBudget float64 `json:"max_daily_budget,omitempty"`
	MinDailyBudget float64 `json:"min_daily_budget,omitempty"`
	Currency       string  `json:"currency,omitempty"`
}

// AdRuleParams is the body of Rules.Create, Rules.Update and Rules.Preview.
// A rule that can raise spend (activate, a positive adjust_budget) needs
// Action.MaxDailyBudget, confirmed by the person as ConfirmMaxDailyBudget,
// and a key with ads:spend (else 403 rule_raises_spend).
// A new rule is off until Enabled is true.
type AdRuleParams struct {
	Name        string            `json:"name,omitempty"`
	AdAccountID AdID              `json:"ad_account_id,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Scope       *AdRuleScope      `json:"scope,omitempty"`
	Conditions  []AdRuleCondition `json:"conditions,omitempty"`
	Action      *AdRuleAction     `json:"action,omitempty"`
	// EveryMinutes is at least 15.
	EveryMinutes          int     `json:"every_minutes,omitempty"`
	CooldownMinutes       int     `json:"cooldown_minutes,omitempty"`
	MaxActionsPerRun      int     `json:"max_actions_per_run,omitempty"`
	ConfirmMaxDailyBudget float64 `json:"confirm_max_daily_budget,omitempty"`
}

// AdRuleActivityParams filters Rules.Activity.
type AdRuleActivityParams struct {
	RuleID AdID `url:"rule_id"`
	Limit  int  `url:"limit"`
}

// ---- Results ------------------------------------------------------------------------

// AdResultRow is one day of your own numbers. Name the campaign by
// CampaignID, ExternalCampaignID + Platform, UTMCampaign or CampaignName.
type AdResultRow struct {
	Day                string  `json:"day"`
	Metric             string  `json:"metric"`
	Value              float64 `json:"value"`
	CampaignID         AdID    `json:"campaign_id,omitempty"`
	ExternalCampaignID string  `json:"external_campaign_id,omitempty"`
	Platform           string  `json:"platform,omitempty"`
	UTMCampaign        string  `json:"utm_campaign,omitempty"`
	CampaignName       string  `json:"campaign_name,omitempty"`
}

// AdResultListParams filters Results.List and Results.Totals (Metric is
// for List only).
type AdResultListParams struct {
	From       string `url:"from"`
	To         string `url:"to"`
	CampaignID AdID   `url:"campaign_id"`
	Metric     string `url:"metric"`
}

// AdResultMetricParams is the body of Results.PutMetric: Unit "number",
// "money" or "percent"; Formula uses numbers, + - * / ( ) and spend,
// impressions, clicks, conversions, leads and results_<key>.
type AdResultMetricParams struct {
	Label   string `json:"label,omitempty"`
	Unit    string `json:"unit,omitempty"`
	Formula string `json:"formula,omitempty"`
}

// ---- Creative tools -----------------------------------------------------------------

// AdMediaListParams is the query of Media.List (Type "image" or "video").
type AdMediaListParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Type        string `url:"type"`
	Cursor      string `url:"cursor"`
}

// AdMediaUploadParams is the body of Media.Upload.
type AdMediaUploadParams struct {
	AdAccountID AdID   `json:"ad_account_id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Name        string `json:"name,omitempty"`
}

// AdMediaRefParams is the query of Media.Get and Media.Delete (Type is
// required on Delete).
type AdMediaRefParams struct {
	AdAccountID AdID   `url:"ad_account_id"`
	Type        string `url:"type"`
}

// AdPreviewParams is the body of Previews.Create. Ad has the shape of a
// draft ad.
type AdPreviewParams struct {
	AdAccountID AdID      `json:"ad_account_id"`
	Ad          AdDraftAd `json:"ad"`
	Placements  []string  `json:"placements,omitempty"`
	Objective   string    `json:"objective,omitempty"`
}

// ---- Account admin ------------------------------------------------------------------

// AdAccountSettingsParams is the body of Accounts.UpdateSettings. A higher
// SpendCap needs ConfirmBudgetAmount = the new cap; SpendCap
// omnisocials.Null removes the cap and needs ConfirmBudgetAmount "remove".
type AdAccountSettingsParams struct {
	SpendCap            any    `json:"spend_cap,omitempty"`
	DSABeneficiary      string `json:"dsa_beneficiary,omitempty"`
	DSAPayor            string `json:"dsa_payor,omitempty"`
	Name                string `json:"name,omitempty"`
	ConfirmBudgetAmount any    `json:"confirm_budget_amount,omitempty"`
}

// AdAccountUserParams is the body of Accounts.AddUser (LinkedIn).
type AdAccountUserParams struct {
	Email    string `json:"email,omitempty"`
	MemberID string `json:"member_id,omitempty"`
	Role     string `json:"role"`
}

// AdAccountActivityParams filters Accounts.Activity.
type AdAccountActivityParams struct {
	Since  string `url:"since"`
	Until  string `url:"until"`
	Cursor string `url:"cursor"`
}

// ---- Meta extras --------------------------------------------------------------------

// AdReachFrequencyParams is the body of ReachFrequency.Create: Objective
// "awareness", "engagement" or "video_views"; exactly one country in
// Targeting; Budget (major units) or Reach. Meta needs the reach and
// frequency capability on the ad account.
type AdReachFrequencyParams struct {
	AdAccountID           AdID                `json:"ad_account_id"`
	Objective             string              `json:"objective"`
	Targeting             AdCampaignTargeting `json:"targeting"`
	StartAt               string              `json:"start_at"`
	EndAt                 string              `json:"end_at"`
	Budget                *AdBudget           `json:"budget,omitempty"`
	Reach                 int                 `json:"reach,omitempty"`
	FrequencyCap          int                 `json:"frequency_cap,omitempty"`
	FrequencyIntervalDays int                 `json:"frequency_interval_days,omitempty"`
}

// AdReachFrequencyReserveParams is the body of ReachFrequency.Reserve:
// ConfirmBudgetAmount = the prediction's budget.
type AdReachFrequencyReserveParams struct {
	AdAccountID         AdID    `json:"ad_account_id"`
	ConfirmBudgetAmount float64 `json:"confirm_budget_amount"`
}

// AdBudgetIncrease is Type "absolute" (Value per day) or "multiplier".
type AdBudgetIncrease struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

// AdBudgetScheduleParams is the body of BudgetSchedules.Create.
// ConfirmBudgetAmount = the increase per day (absolute) or the campaign
// budget times the multiplier.
type AdBudgetScheduleParams struct {
	StartAt             string           `json:"start_at"`
	EndAt               string           `json:"end_at"`
	BudgetIncrease      AdBudgetIncrease `json:"budget_increase"`
	ConfirmBudgetAmount float64          `json:"confirm_budget_amount"`
}

// ---- Google -------------------------------------------------------------------------

// AdGoogleParams is the query of the Google reports. Each report reads the
// fields it documents; the rest stay empty.
type AdGoogleParams struct {
	AdAccountID        AdID   `url:"ad_account_id"`
	CampaignID         AdID   `url:"campaign_id"`
	ExternalCampaignID string `url:"external_campaign_id"`
	ExternalAdSetID    string `url:"external_ad_set_id"`
	From               string `url:"from"`
	To                 string `url:"to"`
	Limit              int    `url:"limit"`
	// Types (recommendations), sent as a comma list.
	Types []string `url:"types"`
	// Type (assets) is "sitelink", "callout" or "structured_snippet".
	Type  string `url:"type"`
	Since string `url:"since"`
}

// ---- Helpers ------------------------------------------------------------------------

func adAccountPath(id AdID, rest string) string {
	return "/ads/accounts/" + url.PathEscape(id.String()) + "/" + rest
}

func adObjectPath(accountID AdID, level, externalID string) string {
	return adAccountPath(accountID, "objects/"+url.PathEscape(level)+"/"+url.PathEscape(externalID))
}

func adAdPath(campaignID, adID AdID) string {
	return adCampaignPath(campaignID) + "/ads/" + url.PathEscape(adID.String())
}

func seg(value string) string { return url.PathEscape(value) }

// queryOf turns a params struct with `url` tags into query values. Zero
// values are left out, a []string is sent as a comma list. A nil pointer
// gives no query.
func queryOf(params any) url.Values {
	q := url.Values{}
	v := reflect.ValueOf(params)
	if !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil()) {
		return q
	}
	v = reflect.Indirect(v)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		key := t.Field(i).Tag.Get("url")
		if key == "" {
			continue
		}
		f := v.Field(i)
		if f.IsZero() {
			continue
		}
		switch f.Kind() {
		case reflect.Slice:
			parts := make([]string, 0, f.Len())
			for j := 0; j < f.Len(); j++ {
				parts = append(parts, fmt.Sprint(f.Index(j).Interface()))
			}
			q.Set(key, strings.Join(parts, ","))
		case reflect.Int, reflect.Int64:
			q.Set(key, strconv.FormatInt(f.Int(), 10))
		case reflect.Bool:
			q.Set(key, strconv.FormatBool(f.Bool()))
		default:
			q.Set(key, fmt.Sprint(f.Interface()))
		}
	}
	return q
}

func accountQuery(adAccountID AdID) url.Values {
	return url.Values{"ad_account_id": {adAccountID.String()}}
}

// adsGet reads `GET path?query` into an ItemResponse.
func adsGet[T any](ctx context.Context, c *Client, path string, query url.Values) (*ItemResponse[T], error) {
	var out ItemResponse[T]
	if err := c.get(ctx, path, query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// adsSend sends a JSON body (nil = none) with the given retry policy.
func adsSend[T any](ctx context.Context, c *Client, method, path string, body any, retry retryPolicy) (*ItemResponse[T], error) {
	var out ItemResponse[T]
	if err := c.doJSONWith(ctx, method, path, body, &out, retry); err != nil {
		return nil, err
	}
	return &out, nil
}

// adsDelete sends `DELETE path?query` without a body.
func adsDelete[T any](ctx context.Context, c *Client, path string, query url.Values) (*ItemResponse[T], error) {
	var out ItemResponse[T]
	if err := c.do(ctx, http.MethodDelete, path, query, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// orEmpty substitutes an empty JSON object for a nil body.
func orEmpty(body any) any {
	v := reflect.ValueOf(body)
	if !v.IsValid() || ((v.Kind() == reflect.Pointer || v.Kind() == reflect.Map) && v.IsNil()) {
		return map[string]any{}
	}
	return body
}

// ---- AdsService (campaign level) ----------------------------------------------------

// BulkStatus calls `POST /ads/campaigns/bulk-status`: pause or resume up to
// 50 campaigns. Pausing needs nothing. Resuming RESTARTS REAL AD SPEND:
// every campaign's budget must equal ConfirmBudgetAmount (one amount for
// all), checked before any change; show the person the total and get their
// yes first. Resuming needs a key with ads:spend: without it, and on Google
// always, a result answers start_in_app. Results per campaign. Not retried
// after a 5xx or a connection error. Requires the ads:write scope.
func (s *AdsService) BulkStatus(ctx context.Context, params *AdBulkStatusParams) (*ItemResponse[AdBulkStatusResults], error) {
	return adsSend[AdBulkStatusResults](ctx, s.client, http.MethodPost, "/ads/campaigns/bulk-status", jsonBody(params), retryRateLimit)
}

// Duplicate calls `POST /ads/campaigns/:id/duplicate`: copy a campaign as a
// new draft (same ad account, goal, budget, targeting, settings and ads).
// An empty name keeps the default. A draft never spends. Requires the
// ads:write scope.
func (s *AdsService) Duplicate(ctx context.Context, id AdID, name string) (*ItemResponse[AdCampaign], error) {
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	return adsSend[AdCampaign](ctx, s.client, http.MethodPost, adCampaignPath(id)+"/duplicate", body, retryAll)
}

// ---- AdObjectsService -----------------------------------------------------------------

// Tree calls `GET /ads/accounts/:accountId/tree`: campaigns, ad sets and ads
// with the numbers of the date range (at most 90 days, default the last 30).
// Each campaign says Source ("omnisocials" or "external"). Requires the
// ads:read scope.
func (s *AdObjectsService) Tree(ctx context.Context, accountID AdID, params *AdTreeParams) (*ItemResponse[AdTree], error) {
	return adsGet[AdTree](ctx, s.client, adAccountPath(accountID, "tree"), queryOf(params))
}

// Get calls `GET /ads/accounts/:accountId/objects/:level/:externalId`: live
// details of one campaign, ad set or ad. Requires the ads:read scope.
func (s *AdObjectsService) Get(ctx context.Context, accountID AdID, level, externalID string) (*ItemResponse[AdTreeNode], error) {
	return adsGet[AdTreeNode](ctx, s.client, adObjectPath(accountID, level, externalID), nil)
}

// Update calls `PATCH /ads/accounts/:accountId/objects/:level/:externalId`:
// change the name, budget or end date of a campaign or ad set made outside
// OmniSocials. A higher budget RAISES REAL AD SPEND: show the person the new
// amount, get their yes, and pass it as ConfirmBudgetAmount. It changes the
// campaign on the network: needs a key with the opt-in ads:spend scope ("Start
// ads"); without it, and on Google (APIDraftOnly) always, the call answers 403
// start_in_app with the page where a person does it in the app. Not retried
// after a 5xx or a connection error: read the object with Get. Requires the
// ads:write scope.
func (s *AdObjectsService) Update(ctx context.Context, accountID AdID, level, externalID string, params *AdObjectUpdateParams) (*ItemResponse[AdTreeNode], error) {
	return adsSend[AdTreeNode](ctx, s.client, http.MethodPatch, adObjectPath(accountID, level, externalID), jsonBody(params), retryRateLimit)
}

// SetStatus calls `PUT /ads/accounts/:accountId/objects/:level/:externalId/status`:
// pause, or switch on. Switching on STARTS REAL AD SPEND: pass the budget
// that pays (the object's own or its parent's) as ConfirmBudgetAmount after
// the person agreed. Switching on: needs a key with the opt-in ads:spend
// scope ("Start ads"); without it, and on Google (APIDraftOnly) always, the
// call answers 403 start_in_app with the page where a person does it in the
// app. Pausing needs nothing and stays open (Google: start_in_app). Not
// retried after a 5xx or a connection error. Requires the ads:write scope.
func (s *AdObjectsService) SetStatus(ctx context.Context, accountID AdID, level, externalID string, params *AdStatusParams) (*ItemResponse[AdObjectStatusResult], error) {
	return adsSend[AdObjectStatusResult](ctx, s.client, http.MethodPut, adObjectPath(accountID, level, externalID)+"/status", jsonBody(params), retryRateLimit)
}

// Duplicate calls `POST /ads/accounts/:accountId/objects/:level/:externalId/duplicate`:
// copy an object made outside OmniSocials; the copy is paused (Meta today).
// It writes on the network: needs a key with the opt-in ads:spend scope
// ("Start ads"); without it, and on Google (APIDraftOnly) always, the call
// answers 403 start_in_app with the page where a person does it in the app.
// Requires the ads:write scope.
func (s *AdObjectsService) Duplicate(ctx context.Context, accountID AdID, level, externalID string, params *AdObjectDuplicateParams) (*ItemResponse[AdTreeNode], error) {
	return adsSend[AdTreeNode](ctx, s.client, http.MethodPost, adObjectPath(accountID, level, externalID)+"/duplicate", jsonBody(params), retryAll)
}

// ImportStatus calls `GET /ads/accounts/:accountId/import`: the import
// state of the campaigns made outside OmniSocials. Requires the ads:read
// scope.
func (s *AdObjectsService) ImportStatus(ctx context.Context, accountID AdID) (*ItemResponse[AdImportState], error) {
	return adsGet[AdImportState](ctx, s.client, adAccountPath(accountID, "import"), nil)
}

// StartImport calls `POST /ads/accounts/:accountId/import`: read the ad
// account again at the next sync (within 15 minutes). The first import reads
// 90 days; webhook ads.initial_sync_completed when it is done. Requires the
// ads:write scope.
func (s *AdObjectsService) StartImport(ctx context.Context, accountID AdID) (*ItemResponse[AdImportState], error) {
	return adsSend[AdImportState](ctx, s.client, http.MethodPost, adAccountPath(accountID, "import"), nil, retryAll)
}

// ListExternal calls `GET /ads/external-campaigns`: the imported campaigns.
// Requires the ads:read scope.
func (s *AdObjectsService) ListExternal(ctx context.Context, params *AdExternalCampaignListParams) (*ItemResponse[AdExternalCampaigns], error) {
	return adsGet[AdExternalCampaigns](ctx, s.client, "/ads/external-campaigns", queryOf(params))
}

// GetExternal calls `GET /ads/external-campaigns/:id`: one imported campaign
// with its daily numbers. Requires the ads:read scope.
func (s *AdObjectsService) GetExternal(ctx context.Context, id AdID, params *AdDateRange) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/external-campaigns/"+seg(id.String()), queryOf(params))
}

// ---- AdCampaignAdsService --------------------------------------------------------------

// List calls `GET /ads/campaigns/:id/ads`: the ads with their status, review
// and latest numbers. Requires the ads:read scope.
func (s *AdCampaignAdsService) List(ctx context.Context, campaignID AdID) (*ItemResponse[AdCampaignAds], error) {
	return adsGet[AdCampaignAds](ctx, s.client, adCampaignPath(campaignID)+"/ads", nil)
}

// Add calls `POST /ads/campaigns/:id/ads`: add up to 6 ads to a campaign that
// runs. The new ads are created PAUSED on the network; switch each one on with
// SetStatus. It changes a campaign on the network: needs a key with the opt-in
// ads:spend scope ("Start ads"); without it, and on Google (APIDraftOnly)
// always, the call answers 403 start_in_app with the page where a person does
// it in the app. Webhook ad.created. Requires the ads:write scope.
func (s *AdCampaignAdsService) Add(ctx context.Context, campaignID AdID, ads []AdDraftAd) (*ItemResponse[AdCampaignAds], error) {
	return adsSend[AdCampaignAds](ctx, s.client, http.MethodPost, adCampaignPath(campaignID)+"/ads", map[string]any{"ads": ads}, retryAll)
}

// Get calls `GET /ads/campaigns/:id/ads/:adId`: one ad. Requires the
// ads:read scope.
func (s *AdCampaignAdsService) Get(ctx context.Context, campaignID, adID AdID) (*ItemResponse[AdCampaignAd], error) {
	return adsGet[AdCampaignAd](ctx, s.client, adAdPath(campaignID, adID), nil)
}

// SetStatus calls `PUT /ads/campaigns/:id/ads/:adId/status`: pause, or switch
// on one ad. Switching on STARTS REAL AD SPEND: pass the campaign budget as
// ConfirmBudgetAmount after the person agreed. Switching on: needs a key with
// the opt-in ads:spend scope ("Start ads"); without it, and on Google
// (APIDraftOnly) always, the call answers 403 start_in_app with the page where
// a person does it in the app. Pausing stays open (Google: start_in_app). Not
// retried after a 5xx or a connection error. Webhook ad.updated. Requires the
// ads:write scope.
func (s *AdCampaignAdsService) SetStatus(ctx context.Context, campaignID, adID AdID, params *AdStatusParams) (*ItemResponse[AdCampaignAd], error) {
	return adsSend[AdCampaignAd](ctx, s.client, http.MethodPut, adAdPath(campaignID, adID)+"/status", jsonBody(params), retryRateLimit)
}

// Review calls `GET /ads/campaigns/:id/ads/:adId/review`: the network's
// review verdict. Requires the ads:read scope.
func (s *AdCampaignAdsService) Review(ctx context.Context, campaignID, adID AdID) (*ItemResponse[AdReview], error) {
	return adsGet[AdReview](ctx, s.client, adAdPath(campaignID, adID)+"/review", nil)
}

// ---- AdCommentsService -----------------------------------------------------------------

// List calls `GET /ads/campaigns/:id/ads/:adId/comments`. An empty cursor
// reads the first page. Requires the ads:read scope.
func (s *AdCommentsService) List(ctx context.Context, campaignID, adID AdID, cursor string) (*ItemResponse[AdsData], error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return adsGet[AdsData](ctx, s.client, adAdPath(campaignID, adID)+"/comments", q)
}

// Reply calls `POST /ads/campaigns/:id/ads/:adId/comments/:commentId/reply`:
// answer a comment. Requires the ads:write scope.
func (s *AdCommentsService) Reply(ctx context.Context, campaignID, adID AdID, commentID, text string) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, adAdPath(campaignID, adID)+"/comments/"+seg(commentID)+"/reply", map[string]any{"text": text}, retryAll)
}

// Hide calls `POST /ads/campaigns/:id/ads/:adId/comments/:commentId/hide`:
// hide (true) or show (false) a comment. Requires the ads:write scope.
func (s *AdCommentsService) Hide(ctx context.Context, campaignID, adID AdID, commentID string, hidden bool) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, adAdPath(campaignID, adID)+"/comments/"+seg(commentID)+"/hide", map[string]any{"hidden": hidden}, retryAll)
}

// Delete calls `DELETE /ads/campaigns/:id/ads/:adId/comments/:commentId`.
// Requires the ads:write scope.
func (s *AdCommentsService) Delete(ctx context.Context, campaignID, adID AdID, commentID string) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, adAdPath(campaignID, adID)+"/comments/"+seg(commentID), nil)
}

// ---- AdTargetingService ----------------------------------------------------------------

// Search calls `GET /ads/targeting/search`: interests, places, languages,
// job titles ... The ids go in AdDraftTargeting.Networks[network].Include.
// Requires the ads:read scope.
func (s *AdTargetingService) Search(ctx context.Context, params *AdTargetingSearchParams) (*ItemResponse[AdTargetingResults], error) {
	return adsGet[AdTargetingResults](ctx, s.client, "/ads/targeting/search", queryOf(params))
}

// Browse calls `GET /ads/targeting/browse`: the categories as a tree.
// Requires the ads:read scope.
func (s *AdTargetingService) Browse(ctx context.Context, params *AdTargetingBrowseParams) (*ItemResponse[AdTargetingResults], error) {
	return adsGet[AdTargetingResults](ctx, s.client, "/ads/targeting/browse", queryOf(params))
}

// ReachEstimate calls `POST /ads/targeting/reach-estimate`: the audience
// size of a targeting. Read-only. Requires the ads:read scope.
func (s *AdTargetingService) ReachEstimate(ctx context.Context, params *AdReachEstimateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/targeting/reach-estimate", jsonBody(params), retryAll)
}

// BidSuggestions calls `POST /ads/targeting/bid-suggestions`: a suggested
// bid and daily budget. Read-only. Requires the ads:read scope.
func (s *AdTargetingService) BidSuggestions(ctx context.Context, params *AdReachEstimateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/targeting/bid-suggestions", jsonBody(params), retryAll)
}

// ---- AdAudiencesService ----------------------------------------------------------------

// List calls `GET /ads/audiences`. Requires the ads:read scope.
func (s *AdAudiencesService) List(ctx context.Context, params *AdAudienceListParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/audiences", queryOf(params))
}

// Create calls `POST /ads/audiences`: a customer list (empty, or with its
// first Schema + Rows), a lookalike or a company list. Requires the
// ads:write scope.
func (s *AdAudiencesService) Create(ctx context.Context, params *AdAudienceCreateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/audiences", jsonBody(params), retryAll)
}

// Get calls `GET /ads/audiences/:audienceId` (the network's audience id).
// Requires the ads:read scope.
func (s *AdAudiencesService) Get(ctx context.Context, audienceID string, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/audiences/"+seg(audienceID), accountQuery(adAccountID))
}

// Update calls `PATCH /ads/audiences/:audienceId`: rename. Requires the
// ads:write scope.
func (s *AdAudiencesService) Update(ctx context.Context, audienceID string, params *AdAudienceUpdateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPatch, "/ads/audiences/"+seg(audienceID), jsonBody(params), retryAll)
}

// Delete calls `DELETE /ads/audiences/:audienceId`. Requires the ads:write
// scope.
func (s *AdAudiencesService) Delete(ctx context.Context, audienceID string, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, "/ads/audiences/"+seg(audienceID), accountQuery(adAccountID))
}

// AddMembers calls `POST /ads/audiences/:audienceId/members`: add people to
// a customer list. Raw values are hashed with SHA-256 before they leave;
// nothing is stored. Requires the ads:write scope.
func (s *AdAudiencesService) AddMembers(ctx context.Context, audienceID string, params *AdAudienceMembersParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/audiences/"+seg(audienceID)+"/members", jsonBody(params), retryAll)
}

// RemoveMembers calls `DELETE /ads/audiences/:audienceId/members` with the
// same body as AddMembers. Requires the ads:write scope.
func (s *AdAudiencesService) RemoveMembers(ctx context.Context, audienceID string, params *AdAudienceMembersParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodDelete, "/ads/audiences/"+seg(audienceID)+"/members", jsonBody(params), retryAll)
}

// AddCompanies calls `POST /ads/audiences/:audienceId/companies`: add
// companies to a company list (LinkedIn). Requires the ads:write scope.
func (s *AdAudiencesService) AddCompanies(ctx context.Context, audienceID string, params *AdAudienceCompaniesParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/audiences/"+seg(audienceID)+"/companies", jsonBody(params), retryAll)
}

// ---- AdLeadFormsService ----------------------------------------------------------------

// List calls `GET /ads/lead-forms`. A form's ExternalID goes in
// AdNetworkSettings.LeadFormID. Requires the ads:read scope.
func (s *AdLeadFormsService) List(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/lead-forms", accountQuery(adAccountID))
}

// Create calls `POST /ads/lead-forms`. Requires the ads:write scope.
func (s *AdLeadFormsService) Create(ctx context.Context, params *AdLeadFormCreateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/lead-forms", jsonBody(params), retryAll)
}

// Get calls `GET /ads/lead-forms/:formId` (the OmniSocials form id).
// Requires the ads:read scope.
func (s *AdLeadFormsService) Get(ctx context.Context, formID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/lead-forms/"+seg(formID.String()), nil)
}

// Update calls `PATCH /ads/lead-forms/:formId`: archive or open a form, or
// switch its lead sync. Requires the ads:write scope.
func (s *AdLeadFormsService) Update(ctx context.Context, formID AdID, params *AdLeadFormUpdateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPatch, "/ads/lead-forms/"+seg(formID.String()), jsonBody(params), retryAll)
}

// Delete calls `DELETE /ads/lead-forms/:formId`: archive a form (the leads
// already copied stay). Requires the ads:write scope.
func (s *AdLeadFormsService) Delete(ctx context.Context, formID AdID) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, "/ads/lead-forms/"+seg(formID.String()), nil)
}

// SendTestLead calls `POST /ads/lead-forms/:formId/test-leads`: a test lead
// (the next sync copies it, marked is_test). Fields may be nil. Requires the
// ads:write scope.
func (s *AdLeadFormsService) SendTestLead(ctx context.Context, formID AdID, fields map[string]any) (*ItemResponse[AdsData], error) {
	body := map[string]any{}
	if fields != nil {
		body["fields"] = fields
	}
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/lead-forms/"+seg(formID.String())+"/test-leads", body, retryAll)
}

// ---- AdLeadsService ---------------------------------------------------------------------

// List calls `GET /ads/leads`: newest first. The answers are personal data.
// Requires the ads:read scope.
func (s *AdLeadsService) List(ctx context.Context, params *AdLeadListParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/leads", queryOf(params))
}

// Counts calls `GET /ads/leads/counts`. A zero formID counts every form.
// Requires the ads:read scope.
func (s *AdLeadsService) Counts(ctx context.Context, formID AdID) (*ItemResponse[AdLeadCounts], error) {
	q := url.Values{}
	if formID != 0 {
		q.Set("form_id", formID.String())
	}
	return adsGet[AdLeadCounts](ctx, s.client, "/ads/leads/counts", q)
}

// Update calls `PATCH /ads/leads/:leadId`: set a lead's status or note.
// Requires the ads:write scope.
func (s *AdLeadsService) Update(ctx context.Context, leadID AdID, params *AdLeadUpdateParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPatch, "/ads/leads/"+seg(leadID.String()), jsonBody(params), retryAll)
}

// ---- AdConversionsService ----------------------------------------------------------------

// Destinations calls `GET /ads/conversions/destinations`: pixels, datasets,
// conversion tags and conversion actions. Requires the ads:read scope.
func (s *AdConversionsService) Destinations(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/conversions/destinations", accountQuery(adAccountID))
}

// Goals calls `GET /ads/conversions/goals`: the events a campaign can
// optimise for. Requires the ads:read scope.
func (s *AdConversionsService) Goals(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/conversions/goals", accountQuery(adAccountID))
}

// CreateCustom calls `POST /ads/conversions/custom`: a custom conversion (a
// URL rule and/or a pixel event). Requires the ads:write scope.
func (s *AdConversionsService) CreateCustom(ctx context.Context, params *AdCustomConversionParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/conversions/custom", jsonBody(params), retryAll)
}

// SendEvents calls `POST /ads/conversions/events`: up to 1,000 conversion
// events from your server. Requires the ads:write scope.
func (s *AdConversionsService) SendEvents(ctx context.Context, params *AdConversionEventsParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/conversions/events", jsonBody(params), retryAll)
}

// ---- AdInsightsService -------------------------------------------------------------------

// Query calls `POST /ads/insights`: live numbers from the network by day,
// ad, country, age, gender, placement, device or platform. Read-only.
// Requires the ads:read scope.
func (s *AdInsightsService) Query(ctx context.Context, params *AdInsightsQueryParams) (*ItemResponse[AdInsightsRows], error) {
	return adsSend[AdInsightsRows](ctx, s.client, http.MethodPost, "/ads/insights", jsonBody(params), retryAll)
}

// Campaign calls `GET /ads/campaigns/:id/analytics`: numbers of an
// OmniSocials campaign (Breakdown "ad" gives one row per ad). Requires the
// ads:read scope.
func (s *AdInsightsService) Campaign(ctx context.Context, campaignID AdID, params *AdCampaignAnalyticsParams) (*ItemResponse[AdInsightsRows], error) {
	return adsGet[AdInsightsRows](ctx, s.client, adCampaignPath(campaignID)+"/analytics", queryOf(params))
}

// Ad calls `GET /ads/campaigns/:id/ads/:adId/analytics`: numbers of one ad
// (From, To and Breakdown of params). Requires the ads:read scope.
func (s *AdInsightsService) Ad(ctx context.Context, campaignID, adID AdID, params *AdCampaignAnalyticsParams) (*ItemResponse[AdInsightsRows], error) {
	return adsGet[AdInsightsRows](ctx, s.client, adAdPath(campaignID, adID)+"/analytics", queryOf(params))
}

// Timeline calls `GET /ads/timeline`: daily totals of the workspace
// (OmniSocials and imported campaigns). Requires the ads:read scope.
func (s *AdInsightsService) Timeline(ctx context.Context, params *AdTimelineParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/timeline", queryOf(params))
}

// Performance calls `GET /ads/performance`: campaigns or ads ranked by one
// metric. Requires the ads:read scope.
func (s *AdInsightsService) Performance(ctx context.Context, params *AdPerformanceParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/performance", queryOf(params))
}

// Alerts calls `GET /ads/alerts`: rejected ads, held campaigns, sign-in or
// payment problems, no delivery. Requires the ads:read scope.
func (s *AdInsightsService) Alerts(ctx context.Context) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/alerts", nil)
}

// ---- AdReportsService --------------------------------------------------------------------

// List calls `GET /ads/insights/reports`: the background reports of the
// workspace. Requires the ads:read scope.
func (s *AdReportsService) List(ctx context.Context) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/insights/reports", nil)
}

// Start calls `POST /ads/insights/reports`: a report of up to 730 days that
// runs in the background (the body of Insights.Query); poll Get. Requires
// the ads:read scope.
func (s *AdReportsService) Start(ctx context.Context, params *AdInsightsQueryParams) (*ItemResponse[AdReport], error) {
	return adsSend[AdReport](ctx, s.client, http.MethodPost, "/ads/insights/reports", jsonBody(params), retryAll)
}

// Get calls `GET /ads/insights/reports/:reportId`. Requires the ads:read
// scope.
func (s *AdReportsService) Get(ctx context.Context, reportID string) (*ItemResponse[AdReport], error) {
	return adsGet[AdReport](ctx, s.client, "/ads/insights/reports/"+seg(reportID), nil)
}

// ---- AdRulesService -----------------------------------------------------------------------

// List calls `GET /ads/rules`. Requires the ads:read scope.
func (s *AdRulesService) List(ctx context.Context) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/rules", nil)
}

// Create calls `POST /ads/rules`: a new rule, off until Enabled is true. A rule
// that can RAISE REAL AD SPEND (activate, a positive adjust_budget) needs the
// person's own ceiling as ConfirmMaxDailyBudget and a key with ads:spend (else
// 403 rule_raises_spend). Preview it first with Preview. Not retried after a
// 5xx or a connection error: read the rules with List. Requires the ads:write
// scope.
func (s *AdRulesService) Create(ctx context.Context, params *AdRuleParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/rules", jsonBody(params), retryRateLimit)
}

// Templates calls `GET /ads/rules/templates`: ready-made rules. Requires
// the ads:read scope.
func (s *AdRulesService) Templates(ctx context.Context) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/rules/templates", nil)
}

// Preview calls `POST /ads/rules/preview`: what an unsaved rule would match
// and do now. Requires the ads:read scope.
func (s *AdRulesService) Preview(ctx context.Context, params *AdRuleParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/rules/preview", jsonBody(params), retryAll)
}

// Activity calls `GET /ads/rules/activity`: every change the rules made,
// with before and after. Requires the ads:read scope.
func (s *AdRulesService) Activity(ctx context.Context, params *AdRuleActivityParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/rules/activity", queryOf(params))
}

// Undo calls `POST /ads/rules/activity/:activityId/undo`: undo a change a
// rule made. Undo of a pause or of a budget cut RESTARTS or RAISES REAL AD
// SPEND: pass the amount as confirmBudgetAmount after the person agreed (0
// leaves it out, for an undo that lowers spend), and it needs a key with
// ads:spend (else, and on Google always, 403 start_in_app). Not retried
// after a 5xx or a connection error. Requires the ads:write scope.
func (s *AdRulesService) Undo(ctx context.Context, activityID AdID, confirmBudgetAmount float64) (*ItemResponse[AdsData], error) {
	body := map[string]any{}
	if confirmBudgetAmount != 0 {
		body["confirm_budget_amount"] = confirmBudgetAmount
	}
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/rules/activity/"+seg(activityID.String())+"/undo", body, retryRateLimit)
}

// Get calls `GET /ads/rules/:ruleId`. Requires the ads:read scope.
func (s *AdRulesService) Get(ctx context.Context, ruleID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/rules/"+seg(ruleID.String()), nil)
}

// Update calls `PATCH /ads/rules/:ruleId`: change a rule (also Enabled). A
// changed action that can raise spend needs ConfirmMaxDailyBudget again and
// a key with ads:spend (else 403 rule_raises_spend).
// Not retried after a 5xx or a connection error. Requires the ads:write
// scope.
func (s *AdRulesService) Update(ctx context.Context, ruleID AdID, params *AdRuleParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPatch, "/ads/rules/"+seg(ruleID.String()), jsonBody(params), retryRateLimit)
}

// Delete calls `DELETE /ads/rules/:ruleId`. Requires the ads:write scope.
func (s *AdRulesService) Delete(ctx context.Context, ruleID AdID) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, "/ads/rules/"+seg(ruleID.String()), nil)
}

// PreviewSaved calls `POST /ads/rules/:ruleId/preview`: what a saved rule
// would match and do now. Requires the ads:read scope.
func (s *AdRulesService) PreviewSaved(ctx context.Context, ruleID AdID) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/rules/"+seg(ruleID.String())+"/preview", nil, retryAll)
}

// Runs calls `GET /ads/rules/:ruleId/runs`. A zero limit uses the default.
// Requires the ads:read scope.
func (s *AdRulesService) Runs(ctx context.Context, ruleID AdID, limit int) (*ItemResponse[AdsData], error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return adsGet[AdsData](ctx, s.client, "/ads/rules/"+seg(ruleID.String())+"/runs", q)
}

// ---- AdResultsService ---------------------------------------------------------------------

// List calls `GET /ads/campaign-results`. Requires the ads:read scope.
func (s *AdResultsService) List(ctx context.Context, params *AdResultListParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/campaign-results", queryOf(params))
}

// Upload calls `POST /ads/campaign-results`: up to 5,000 rows; sending a day
// again replaces it. Requires the ads:write scope.
func (s *AdResultsService) Upload(ctx context.Context, rows []AdResultRow) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/campaign-results", map[string]any{"rows": rows}, retryAll)
}

// Totals calls `GET /ads/campaign-results/totals`: totals over a period,
// with the formulas. Requires the ads:read scope.
func (s *AdResultsService) Totals(ctx context.Context, params *AdResultListParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/campaign-results/totals", queryOf(params))
}

// Metrics calls `GET /ads/campaign-results/metrics`: your metric names and
// formulas. Requires the ads:read scope.
func (s *AdResultsService) Metrics(ctx context.Context) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/campaign-results/metrics", nil)
}

// PutMetric calls `PUT /ads/campaign-results/metrics/:key`: name a metric or
// define a formula. Requires the ads:write scope.
func (s *AdResultsService) PutMetric(ctx context.Context, key string, params *AdResultMetricParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPut, "/ads/campaign-results/metrics/"+seg(key), jsonBody(params), retryAll)
}

// ---- AdMediaService -----------------------------------------------------------------------

// List calls `GET /ads/media`: the network's own image or video library.
// Requires the ads:read scope.
func (s *AdMediaService) List(ctx context.Context, params *AdMediaListParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/media", queryOf(params))
}

// Upload calls `POST /ads/media`: upload an image or video to the network
// ahead of an ad; use the id as the ad's network media_ref. Requires the
// ads:write scope.
func (s *AdMediaService) Upload(ctx context.Context, params *AdMediaUploadParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/media", jsonBody(params), retryAll)
}

// Get calls `GET /ads/media/:mediaId`: the processing status of an upload.
// Requires the ads:read scope.
func (s *AdMediaService) Get(ctx context.Context, mediaID string, params *AdMediaRefParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/media/"+seg(mediaID), queryOf(params))
}

// Delete calls `DELETE /ads/media/:mediaId` (Type is required). Requires the
// ads:write scope.
func (s *AdMediaService) Delete(ctx context.Context, mediaID string, params *AdMediaRefParams) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, "/ads/media/"+seg(mediaID), queryOf(params))
}

// ---- AdPreviewsService --------------------------------------------------------------------

// Create calls `POST /ads/previews`: previews of a new ad drawn by the
// network. Read-only. Requires the ads:read scope.
func (s *AdPreviewsService) Create(ctx context.Context, params *AdPreviewParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/previews", jsonBody(params), retryAll)
}

// ForAd calls `GET /ads/campaigns/:id/ads/:adId/previews`: previews of an
// existing ad (placements may be empty). Requires the ads:read scope.
func (s *AdPreviewsService) ForAd(ctx context.Context, campaignID, adID AdID, placements []string) (*ItemResponse[AdsData], error) {
	q := url.Values{}
	if len(placements) > 0 {
		q.Set("placements", strings.Join(placements, ","))
	}
	return adsGet[AdsData](ctx, s.client, adAdPath(campaignID, adID)+"/previews", q)
}

// ---- AdPartnershipService -----------------------------------------------------------------

// ListContent calls `GET /ads/partnership/content`: creator posts you may
// run as partnership ads (Meta). Requires the ads:read scope.
func (s *AdPartnershipService) ListContent(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/partnership/content", accountQuery(adAccountID))
}

// ListPermissions calls `GET /ads/partnership/permissions` (Meta). Requires
// the ads:read scope.
func (s *AdPartnershipService) ListPermissions(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/partnership/permissions", accountQuery(adAccountID))
}

// SetPermission calls `POST /ads/partnership/permissions`: ask for or set a
// permission (Meta). params passes Meta's fields through and must carry
// "ad_account_id". Requires the ads:write scope.
func (s *AdPartnershipService) SetPermission(ctx context.Context, params map[string]any) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/partnership/permissions", orEmpty(params), retryAll)
}

// ---- AdAccountsService --------------------------------------------------------------------

// Identities calls `GET /ads/accounts/:accountId/identities`: Pages,
// profiles and identities an ad can run as. Requires the ads:read scope.
func (s *AdAccountsService) Identities(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "identities"), nil)
}

// Health calls `GET /ads/accounts/:accountId/health`: status, payment, spend
// limit, Page and pixel before an ad starts. A check that cannot be read is
// "unknown"; failed checks carry a fix_url. Requires the ads:read scope.
func (s *AdAccountsService) Health(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "health"), nil)
}

// Finance calls `GET /ads/accounts/:accountId/finance`: spend limit, amount
// spent, balance and payment method. Requires the ads:read scope.
func (s *AdAccountsService) Finance(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "finance"), nil)
}

// UpdateSettings calls `PATCH /ads/accounts/:accountId/settings`: spend cap,
// DSA payer and beneficiary, name. A higher spend cap ALLOWS MORE REAL AD
// SPEND: pass the new cap as ConfirmBudgetAmount after the person agreed
// (removing the cap: SpendCap omnisocials.Null and ConfirmBudgetAmount
// "remove"). A spend cap change: needs a key with the opt-in ads:spend scope
// ("Start ads"); without it, and on Google (APIDraftOnly) always, the call
// answers 403 start_in_app with the page where a person does it in the app. The
// name and the DSA names stay open. Not retried after a 5xx or a connection
// error. Requires the ads:write scope.
func (s *AdAccountsService) UpdateSettings(ctx context.Context, accountID AdID, params *AdAccountSettingsParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPatch, adAccountPath(accountID, "settings"), jsonBody(params), retryRateLimit)
}

// ListUsers calls `GET /ads/accounts/:accountId/users`: people with access.
// Requires the ads:read scope.
func (s *AdAccountsService) ListUsers(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "users"), nil)
}

// AddUser calls `POST /ads/accounts/:accountId/users`: give a person access
// (LinkedIn). Requires the ads:write scope.
func (s *AdAccountsService) AddUser(ctx context.Context, accountID AdID, params *AdAccountUserParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, adAccountPath(accountID, "users"), jsonBody(params), retryAll)
}

// RemoveUser calls `DELETE /ads/accounts/:accountId/users/:userId`: remove a
// person's access. Requires the ads:write scope.
func (s *AdAccountsService) RemoveUser(ctx context.Context, accountID AdID, userID string) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, adAccountPath(accountID, "users/"+seg(userID)), nil)
}

// Activity calls `GET /ads/accounts/:accountId/activity`: the change log of
// the ad account. Requires the ads:read scope.
func (s *AdAccountsService) Activity(ctx context.Context, accountID AdID, params *AdAccountActivityParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "activity"), queryOf(params))
}

// Businesses calls `GET /ads/accounts/:accountId/businesses`: Meta business
// portfolios and TikTok Business Centers. Requires the ads:read scope.
func (s *AdAccountsService) Businesses(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "businesses"), nil)
}

// DSA calls `GET /ads/accounts/:accountId/dsa`: the EU DSA payer and
// beneficiary (Meta). Requires the ads:read scope.
func (s *AdAccountsService) DSA(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "dsa"), nil)
}

// Hierarchy calls `GET /ads/accounts/:accountId/hierarchy`: the manager
// account hierarchy (Google). Requires the ads:read scope.
func (s *AdAccountsService) Hierarchy(ctx context.Context, accountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adAccountPath(accountID, "hierarchy"), nil)
}

// ---- AdReachFrequencyService --------------------------------------------------------------

// Create calls `POST /ads/reach-frequency`: a prediction (Meta). A
// prediction spends nothing; Reserve does. Requires the ads:write scope.
func (s *AdReachFrequencyService) Create(ctx context.Context, params *AdReachFrequencyParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/reach-frequency", jsonBody(params), retryAll)
}

// Get calls `GET /ads/reach-frequency/:predictionId`. Requires the ads:read
// scope.
func (s *AdReachFrequencyService) Get(ctx context.Context, predictionID string, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/reach-frequency/"+seg(predictionID), accountQuery(adAccountID))
}

// Cancel calls `DELETE /ads/reach-frequency/:predictionId`: cancel a
// prediction or a reservation. Requires the ads:write scope.
func (s *AdReachFrequencyService) Cancel(ctx context.Context, predictionID string, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, "/ads/reach-frequency/"+seg(predictionID), accountQuery(adAccountID))
}

// Reserve calls `POST /ads/reach-frequency/:predictionId/reserve`: it COMMITS
// REAL AD SPEND. Pass the prediction's budget as ConfirmBudgetAmount after the
// person agreed. Needs a key with the opt-in ads:spend scope ("Start ads");
// without it, and on Google (APIDraftOnly) always, the call answers 403
// start_in_app with the page where a person does it in the app. Not retried
// after a 5xx or a connection error: read the prediction with Get. Requires the
// ads:write scope.
func (s *AdReachFrequencyService) Reserve(ctx context.Context, predictionID string, params *AdReachFrequencyReserveParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/reach-frequency/"+seg(predictionID)+"/reserve", jsonBody(params), retryRateLimit)
}

// ---- AdStudiesService ---------------------------------------------------------------------

// List calls `GET /ads/studies`: A/B tests and lift studies (Meta).
// Requires the ads:read scope.
func (s *AdStudiesService) List(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/studies", accountQuery(adAccountID))
}

// Create calls `POST /ads/studies`: an A/B test between existing campaigns or
// ad sets (Meta). params passes Meta's fields through and must carry
// "ad_account_id". Needs a key with the opt-in ads:spend scope ("Start ads");
// without it, and on Google (APIDraftOnly) always, the call answers 403
// start_in_app with the page where a person does it in the app. Requires the
// ads:write scope.
func (s *AdStudiesService) Create(ctx context.Context, params map[string]any) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, "/ads/studies", orEmpty(params), retryAll)
}

// ---- AdBudgetSchedulesService ---------------------------------------------------------------

// List calls `GET /ads/campaigns/:id/budget-schedules`. Requires the
// ads:read scope.
func (s *AdBudgetSchedulesService) List(ctx context.Context, campaignID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, adCampaignPath(campaignID)+"/budget-schedules", nil)
}

// Create calls `POST /ads/campaigns/:id/budget-schedules`: plan a budget
// increase, which is NEW REAL AD SPEND in that period. Pass the increase per
// day (absolute) or the campaign budget times the multiplier as
// ConfirmBudgetAmount after the person agreed. Needs a key with the opt-in
// ads:spend scope ("Start ads"); without it, and on Google (APIDraftOnly)
// always, the call answers 403 start_in_app with the page where a person does
// it in the app. Not retried after a 5xx or a connection error: read the
// schedules with List. Requires the ads:write scope.
func (s *AdBudgetSchedulesService) Create(ctx context.Context, campaignID AdID, params *AdBudgetScheduleParams) (*ItemResponse[AdsData], error) {
	return adsSend[AdsData](ctx, s.client, http.MethodPost, adCampaignPath(campaignID)+"/budget-schedules", jsonBody(params), retryRateLimit)
}

// Delete calls `DELETE /ads/campaigns/:id/budget-schedules/:scheduleId`.
// Requires the ads:write scope.
func (s *AdBudgetSchedulesService) Delete(ctx context.Context, campaignID AdID, scheduleID string) (*ItemResponse[AdsData], error) {
	return adsDelete[AdsData](ctx, s.client, adCampaignPath(campaignID)+"/budget-schedules/"+seg(scheduleID), nil)
}

// ---- AdGoogleService ------------------------------------------------------------------------

// SearchTerms calls `GET /ads/google/search-terms` (AdAccountID,
// CampaignID or ExternalCampaignID, From, To, Limit). Requires the ads:read
// scope.
func (s *AdGoogleService) SearchTerms(ctx context.Context, params *AdGoogleParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/search-terms", queryOf(params))
}

// Recommendations calls `GET /ads/google/recommendations` (AdAccountID,
// Types). Requires the ads:read scope.
func (s *AdGoogleService) Recommendations(ctx context.Context, params *AdGoogleParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/recommendations", queryOf(params))
}

// Keywords calls `GET /ads/google/keywords`: search keywords and negative
// keywords (AdAccountID, ExternalCampaignID, ExternalAdSetID). Requires the
// ads:read scope.
func (s *AdGoogleService) Keywords(ctx context.Context, params *AdGoogleParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/keywords", queryOf(params))
}

// Assets calls `GET /ads/google/assets`: sitelinks, callouts and structured
// snippets (AdAccountID, Type). Requires the ads:read scope.
func (s *AdGoogleService) Assets(ctx context.Context, params *AdGoogleParams) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/assets", queryOf(params))
}

// SharedBudgets calls `GET /ads/google/shared-budgets`. Requires the
// ads:read scope.
func (s *AdGoogleService) SharedBudgets(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/shared-budgets", accountQuery(adAccountID))
}

// BidStrategies calls `GET /ads/google/bid-strategies`: portfolio bid
// strategies. Requires the ads:read scope.
func (s *AdGoogleService) BidStrategies(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/bid-strategies", accountQuery(adAccountID))
}

// AdSchedule calls `GET /ads/google/ad-schedule`: the days and hours of a
// Google campaign. Requires the ads:read scope.
func (s *AdGoogleService) AdSchedule(ctx context.Context, adAccountID AdID, externalCampaignID string) (*ItemResponse[AdsData], error) {
	q := accountQuery(adAccountID)
	q.Set("external_campaign_id", externalCampaignID)
	return adsGet[AdsData](ctx, s.client, "/ads/google/ad-schedule", q)
}

// Labels calls `GET /ads/google/labels`. Requires the ads:read scope.
func (s *AdGoogleService) Labels(ctx context.Context, adAccountID AdID) (*ItemResponse[AdsData], error) {
	return adsGet[AdsData](ctx, s.client, "/ads/google/labels", accountQuery(adAccountID))
}

// LocalServicesLeads calls `GET /ads/google/local-services-leads` (an empty
// since reads the default period). Requires the ads:read scope.
func (s *AdGoogleService) LocalServicesLeads(ctx context.Context, adAccountID AdID, since string) (*ItemResponse[AdsData], error) {
	q := accountQuery(adAccountID)
	if since != "" {
		q.Set("since", since)
	}
	return adsGet[AdsData](ctx, s.client, "/ads/google/local-services-leads", q)
}
