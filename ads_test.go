package omnisocials

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

const testAdDraftBody = `{"data":{"group_id":"g-1","name":"Autumn sale","objective":"traffic","status":"draft",
	"can_launch":true,"budget":{"type":"daily","amount":10,"currency":"EUR"},"schedule":{"duration_days":7},
	"targeting":{"countries":["NL"],"automatic_audience":true,"placements":"automatic"},
	"ads":[{"key":"a1","media":[{"url":"https://example.com/sale.jpg","type":"image"}],"primary_text":"30% off","link":"https://example.com"}],
	"campaigns":[{"id":"7","platform":"meta","status":"draft","editable":true,"external_campaign_id":null,
		"budget":{"type":"daily","amount":10,"currency":"EUR"},
		"ad_account":{"id":"12","name":"Shop","currency":"EUR","billing_ok":null,"needs_reauth":false},"errors":null,
		"readiness":{"ready":true,"issues":[],"ads":[{"key":"a1","index":0,"ready":true,"issues":[]}]}}],
	"review_url":"https://app.omnisocials.com/ads/create/g-1"}}`

func testAdDraftParams() *AdDraftParams {
	return &AdDraftParams{
		Name:      "Autumn sale",
		Objective: "traffic",
		Platforms: []string{"meta"},
		Budget:    AdBudget{Amount: 10, Currency: "EUR"},
		Schedule:  &AdDraftSchedule{DurationDays: Int(7)},
		Targeting: AdDraftTargeting{Countries: []string{"NL"}, AutomaticAudience: Bool(true)},
		Ads: []AdDraftAd{{
			Media:       []AdMediaFile{{URL: "https://example.com/sale.jpg", Type: "image"}},
			PrimaryText: "30% off",
			Link:        "https://example.com",
			PlacementMedia: &AdPlacementMedia{
				StoriesReels: &AdMediaFile{URL: "https://example.com/sale-9x16.jpg", Type: "image"},
			},
		}},
	}
}

type capturedRequest struct {
	method, path string
	body         map[string]any
}

func captureAll(t *testing.T, requests *[]capturedRequest, r *http.Request) {
	t.Helper()
	data, _ := io.ReadAll(r.Body)
	var body map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("request body is not JSON: %s", data)
		}
	}
	*requests = append(*requests, capturedRequest{r.Method, r.URL.EscapedPath(), body})
}

func TestAdsDraftCreateUpdateLaunchAndActivateRequests(t *testing.T) {
	var requests []capturedRequest
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captureAll(t, &requests, r)
		switch {
		case strings.HasSuffix(r.URL.Path, "/launch"):
			fmt.Fprint(w, `{"data":{"group":`+strings.TrimSuffix(strings.TrimPrefix(testAdDraftBody, `{"data":`), `}`)+`,
				"results":[{"campaign_id":"7","platform":"meta","ok":true}]}}`)
		case strings.HasSuffix(r.URL.Path, "/activate"):
			fmt.Fprint(w, `{"data":{"id":"7","name":"Boost","platform":"meta","campaign_type":"boost","status":"pending_review",
				"budget":{"type":"daily","amount":25,"currency":"EUR"},"ad_account_id":"12","external_campaign_id":"2385"}}`)
		default:
			fmt.Fprint(w, testAdDraftBody)
		}
	}), WithMaxRetries(0))
	ctx := context.Background()

	created, err := client.Ads.Drafts.Create(ctx, testAdDraftParams())
	if err != nil {
		t.Fatalf("Ads.Drafts.Create: %v", err)
	}
	if created.Data.GroupID != "g-1" || created.Data.Schedule.DurationDays == nil || *created.Data.Schedule.DurationDays != 7 ||
		len(created.Data.Campaigns) != 1 || created.Data.Campaigns[0].ID != 7 ||
		created.Data.Campaigns[0].AdAccount == nil || created.Data.Campaigns[0].AdAccount.ID != 12 ||
		created.Data.Campaigns[0].AdAccount.BillingOK != nil || !created.Data.Campaigns[0].Readiness.Ready {
		t.Fatalf("unexpected draft: %+v", created.Data)
	}

	if _, err := client.Ads.Drafts.Update(ctx, "g-1", testAdDraftParams()); err != nil {
		t.Fatalf("Ads.Drafts.Update: %v", err)
	}
	launch, err := client.Ads.Drafts.Launch(ctx, "g-1", &AdDraftLaunchParams{ConfirmBudgetAmount: 10})
	if err != nil {
		t.Fatalf("Ads.Drafts.Launch: %v", err)
	}
	if len(launch.Data.Results) != 1 || launch.Data.Results[0].CampaignID != 7 || !launch.Data.Results[0].OK ||
		launch.Data.Group.GroupID != "g-1" {
		t.Fatalf("unexpected launch result: %+v", launch.Data)
	}
	activated, err := client.Ads.Activate(ctx, 7, 25)
	if err != nil {
		t.Fatalf("Ads.Activate: %v", err)
	}
	if activated.Data.ID != 7 || activated.Data.Status != "pending_review" ||
		activated.Data.AdAccountID == nil || *activated.Data.AdAccountID != 12 {
		t.Fatalf("unexpected campaign: %+v", activated.Data)
	}

	if len(requests) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(requests))
	}
	want := []struct{ method, path string }{
		{http.MethodPost, "/ads/drafts"},
		{http.MethodPut, "/ads/drafts/g-1"},
		{http.MethodPost, "/ads/drafts/g-1/launch"},
		{http.MethodPost, "/ads/campaigns/7/activate"},
	}
	for i, w := range want {
		if requests[i].method != w.method || requests[i].path != w.path {
			t.Fatalf("request %d: expected %s %s, got %s %s", i, w.method, w.path, requests[i].method, requests[i].path)
		}
	}

	// The draft body: the full draft, the same on create and update.
	draftJSON := `{"name":"Autumn sale","objective":"traffic","platforms":["meta"],
		"budget":{"amount":10,"currency":"EUR"},"schedule":{"duration_days":7},
		"targeting":{"countries":["NL"],"automatic_audience":true},
		"ads":[{"media":[{"url":"https://example.com/sale.jpg","type":"image"}],"primary_text":"30% off",
		"link":"https://example.com","placement_media":{"stories_reels":{"url":"https://example.com/sale-9x16.jpg","type":"image"}}}]}`
	var wantDraft map[string]any
	if err := json.Unmarshal([]byte(draftJSON), &wantDraft); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, _ := json.Marshal(requests[i].body)
		exp, _ := json.Marshal(wantDraft)
		if string(got) != string(exp) {
			t.Fatalf("request %d body:\n got  %s\n want %s", i, got, exp)
		}
	}
	if fmt.Sprint(requests[2].body) != "map[confirm_budget_amount:10]" {
		t.Fatalf("unexpected launch body %v", requests[2].body)
	}
	if fmt.Sprint(requests[3].body) != "map[confirm_budget_amount:25]" {
		t.Fatalf("unexpected activate body %v", requests[3].body)
	}
}

func TestAdsLaunchPerCampaignAmounts(t *testing.T) {
	var gotBody string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		fmt.Fprint(w, `{"data":{"group":{"group_id":"g-1"},"results":[]}}`)
	}))
	_, err := client.Ads.Drafts.Launch(context.Background(), "g-1", &AdDraftLaunchParams{
		ConfirmBudgetAmounts: map[AdID]float64{7: 10, 8: 20},
	})
	if err != nil {
		t.Fatalf("Ads.Drafts.Launch: %v", err)
	}
	if gotBody != `{"confirm_budget_amounts":{"7":10,"8":20}}` {
		t.Fatalf("unexpected body %s", gotBody)
	}
}

// A call that can start or raise ad spend may have gone through when the API
// answers 5xx, so it is sent exactly once.
func TestAdsMoneyCallsAreNotRetriedAfter5xx(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"error":{"code":"bad_gateway","message":"upstream"}}`)
	}), WithMaxRetries(2))
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"Drafts.Launch", func() error {
			_, err := client.Ads.Drafts.Launch(ctx, "g-1", &AdDraftLaunchParams{ConfirmBudgetAmount: 10})
			return err
		}},
		{"Activate", func() error { _, err := client.Ads.Activate(ctx, 7, 25); return err }},
		{"ActivateMulti", func() error { _, err := client.Ads.ActivateMulti(ctx, []AdID{7, 8}, 25); return err }},
		{"Resume", func() error { _, err := client.Ads.Resume(ctx, 7); return err }},
		{"Update", func() error {
			_, err := client.Ads.Update(ctx, 7, &AdCampaignUpdateParams{
				Budget: &AdBudget{Amount: 40, Currency: "EUR"}, ConfirmBudgetAmount: 40,
			})
			return err
		}},
	}
	for _, tc := range cases {
		calls.Store(0)
		err := tc.call()
		assertErrorAs[*ServerError](t, err)
		if got := calls.Load(); got != 1 {
			t.Fatalf("%s must be sent once after a 502, got %d attempts", tc.name, got)
		}
	}
}

func TestAdsMoneyCallIsNotRetriedAfterAConnectionError(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close() // drop the connection without an answer
	}), WithMaxRetries(2))

	_, err := client.Ads.Activate(context.Background(), 7, 25)
	assertErrorAs[*ConnectionError](t, err)
	if got := calls.Load(); got != 1 {
		t.Fatalf("an activation must be sent once after a dropped connection, got %d attempts", got)
	}
}

func TestAdsMoneyCallIsRetriedAfter429(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"rate_limit_exceeded","message":"slow down"}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"group":{"group_id":"g-1"},"results":[]}}`)
	}), WithMaxRetries(2))

	if _, err := client.Ads.Drafts.Launch(context.Background(), "g-1", &AdDraftLaunchParams{ConfirmBudgetAmount: 10}); err != nil {
		t.Fatalf("expected the 429 to be retried, got: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestAdsReadIsRetriedAfter5xx(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, `{"error":{"code":"bad_gateway","message":"upstream"}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"networks":[{"key":"meta","label":"Meta","runs_on":"Instagram and Facebook",
			"identity":"The Page","goals":{"traffic":{"ok":true},"engagement":{"ok":"approx","note":"clicks"}},
			"media":{"image":{"ok":true}},"text":{"headline":{"max":255,"shown":40,"required":true},
			"description":{"max":0,"shown":0,"required":false,"field":false}},"ctas":["learn_more"],"no_cta":false,
			"extra_fields":[],"min_daily_budget":{"default":20,"byCurrency":{"SEK":200}},"min_duration_days":null,
			"max_ads_per_campaign":null,"review":"24 hours"}],"goals":["traffic"],
			"ctas":[{"key":"learn_more","label":"Learn more"}],"create_networks":["meta"],"network_states":{"meta":"live"}}}`)
	}), WithMaxRetries(1))

	caps, err := client.Ads.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("expected the read to be retried, got: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
	meta := caps.Data.Networks[0]
	if meta.Goals["engagement"].OK != "approx" || meta.Goals["traffic"].OK != true ||
		meta.Text["description"].Field == nil || *meta.Text["description"].Field ||
		meta.MinDailyBudget.ByCurrency["SEK"] != 200 || caps.Data.NetworkStates["meta"] != "live" {
		t.Fatalf("unexpected capabilities: %+v", caps.Data)
	}
}

func TestAdsListQueryAndBoostBody(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotQuery, gotBody = r.Method, r.URL.Path, r.URL.RawQuery, string(data)
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"data":[{"id":7,"name":"Boost","platform":"meta","campaign_type":"boost","status":"active",
				"metrics":{"spend":12.5,"impressions":1000}}],"pagination":{"total":1,"limit":10,"offset":0,"has_more":false}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"id":"9","name":"Boost","platform":"meta","campaign_type":"boost","status":"draft"}}`)
	}))
	ctx := context.Background()

	list, err := client.Ads.List(ctx, &AdCampaignListParams{Status: "active", Platform: "meta", CampaignType: "boost", Limit: 10})
	if err != nil {
		t.Fatalf("Ads.List: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/ads/campaigns" ||
		gotQuery != "campaign_type=boost&limit=10&platform=meta&status=active" {
		t.Fatalf("unexpected list request %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	// A numeric id decodes as well as a string one.
	if len(list.Data) != 1 || list.Data[0].ID != 7 || list.Data[0].Metrics["spend"] != 12.5 {
		t.Fatalf("unexpected campaigns: %+v", list.Data)
	}

	boost, err := client.Ads.Boost(ctx, &AdBoostParams{
		PostID:      "12345",
		AdAccountID: 12,
		Budget:      AdBudget{Type: "daily", Amount: 5, Currency: "EUR"},
		Targeting:   map[string]any{"countries": []string{"NL"}},
		Creative:    &AdCreative{SparkAuthCode: "spark"},
	})
	if err != nil {
		t.Fatalf("Ads.Boost: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/ads/boost" ||
		gotBody != `{"post_id":"12345","ad_account_id":12,"budget":{"type":"daily","amount":5,"currency":"EUR"},"targeting":{"countries":["NL"]},"creative":{"spark_auth_code":"spark"}}` {
		t.Fatalf("unexpected boost request %s %s %s", gotMethod, gotPath, gotBody)
	}
	if boost.Data.ID != 9 || boost.Data.Status != "draft" {
		t.Fatalf("unexpected boost: %+v", boost.Data)
	}
}

// The API sends ad ids as JSON strings (database BIGINT) and a campaign's
// timestamps in camelCase; an ad account keeps created_at.
func TestAdCampaignDecodesStringIDsAndCamelCaseTimestamps(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ads/accounts":
			fmt.Fprint(w, `{"data":[{"id":"12","workspace_id":456,"platform":"meta","external_account_id":"act_1",
				"account_info":{"name":"Shop","currency":"EUR","billing_ok":null},"is_selected":true,"status":"active",
				"needs_reauth":false,"created_at":"2026-10-01T09:00:00.000Z"}],"platforms_available":["meta"]}`)
		default:
			fmt.Fprint(w, `{"data":{"id":"51","ad_account_id":"12","name":"Boost","platform":"meta","campaign_type":"boost",
				"status":"draft","budget":{"type":"daily","amount":5,"currency":"EUR"},"external_campaign_id":null,
				"errors":null,"group_id":null,"createdAt":"2026-10-28T10:00:00.000Z","updatedAt":"2026-10-28T10:05:00.000Z"}}`)
		}
	}))
	ctx := context.Background()

	campaign, err := client.Ads.Get(ctx, 51)
	if err != nil {
		t.Fatalf("Ads.Get: %v", err)
	}
	c := campaign.Data
	if c.ID != 51 || c.AdAccountID == nil || *c.AdAccountID != 12 || c.ExternalCampaignID != nil ||
		c.CreatedAt != "2026-10-28T10:00:00.000Z" || c.UpdatedAt != "2026-10-28T10:05:00.000Z" {
		t.Fatalf("unexpected campaign: %+v", c)
	}

	accounts, err := client.Ads.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("Ads.ListAccounts: %v", err)
	}
	a := accounts.Data[0]
	if a.ID != 12 || a.CreatedAt != "2026-10-01T09:00:00.000Z" || a.AccountInfo == nil ||
		a.AccountInfo.BillingOK != nil || len(accounts.PlatformsAvailable) != 1 {
		t.Fatalf("unexpected account: %+v", a)
	}
}

func TestAdIDDecodesNumbersAndStrings(t *testing.T) {
	var ids []AdID
	if err := json.Unmarshal([]byte(`[7,"8",null]`), &ids); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(ids) != 3 || ids[0] != 7 || ids[1] != 8 || ids[2] != 0 {
		t.Fatalf("unexpected ids %v", ids)
	}
	var bad AdID
	if err := json.Unmarshal([]byte(`"abc"`), &bad); err == nil {
		t.Fatal("expected an error for a non-numeric id")
	}
	if out, _ := json.Marshal(AdID(42)); string(out) != "42" {
		t.Fatalf("an id encodes as a number, got %s", out)
	}
}

// Several currencies: spend is null and totals_by_currency holds each one;
// capabilities name the networks where every API caller stops at the draft
// (also a key with ads:spend) and the ones where even a pause or a delete
// happens in the app (both Google).
func TestAdsOverviewPerCurrencyAndCapabilitiesStopLists(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ads/overview":
			fmt.Fprint(w, `{"data":{"connected_accounts":2,"campaigns_total":3,"campaigns_by_status":{"active":2},
				"totals":{"spend":null,"currency":null,"impressions":3000,"clicks":40},
				"totals_by_currency":[{"currency":"EUR","spend":10.5,"impressions":1000,"clicks":15},
				{"currency":"JPY","spend":1500,"impressions":2000,"clicks":25}]}}`)
		default:
			fmt.Fprint(w, `{"data":{"networks":[],"goals":[],"ctas":[],"create_networks":["meta"],
				"api_draft_only":["google"],"api_stop_in_app":["google"]}}`)
		}
	}))
	ctx := context.Background()

	overview, err := client.Ads.Overview(ctx)
	if err != nil {
		t.Fatalf("Ads.Overview: %v", err)
	}
	o := overview.Data
	if o.Totals.Spend != nil || o.Totals.Currency != nil || o.Totals.Impressions != 3000 ||
		len(o.TotalsByCurrency) != 2 || o.TotalsByCurrency[1].Currency == nil ||
		*o.TotalsByCurrency[1].Currency != "JPY" || o.TotalsByCurrency[1].Spend != 1500 {
		t.Fatalf("unexpected overview: %+v", o)
	}

	caps, err := client.Ads.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Ads.Capabilities: %v", err)
	}
	if len(caps.Data.APIDraftOnly) != 1 || caps.Data.APIDraftOnly[0] != "google" ||
		len(caps.Data.APIStopInApp) != 1 || caps.Data.APIStopInApp[0] != "google" {
		t.Fatalf("unexpected capabilities: %+v", caps.Data)
	}
}

// POST /ads/campaigns is gone (a campaign without ads could never start):
// AdsService has no Create method any more. New ads go through Drafts.Create.
func TestAdsCampaignCreateIsGone(t *testing.T) {
	if _, ok := reflect.TypeOf(&AdsService{}).MethodByName("Create"); ok {
		t.Fatal("AdsService.Create was removed: POST /ads/campaigns no longer exists")
	}
}
