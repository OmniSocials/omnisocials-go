package omnisocials

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
)

type parityRequest struct {
	method, path, query string
	body                map[string]any
	rawBody             string
}

func parityClient(t *testing.T, requests *[]parityRequest, answer string) *Client {
	t.Helper()
	return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		if len(data) > 0 {
			if err := json.Unmarshal(data, &body); err != nil {
				t.Errorf("request body is not JSON: %s", data)
			}
		}
		*requests = append(*requests, parityRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, body, string(data)})
		fmt.Fprint(w, answer)
	}), WithMaxRetries(0))
}

func TestAdsParityServicesAreWired(t *testing.T) {
	client, err := NewClient(WithAPIKey("omsk_test_key"))
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(client.Ads).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Pointer && v.Type().Field(i).IsExported() && f.IsNil() {
			t.Fatalf("client.Ads.%s is nil", v.Type().Field(i).Name)
		}
	}
	// One method per route of backend/routes/api/v1/adsParity.js.
	counts := map[string]int{
		"Objects": 9, "CampaignAds": 5, "Comments": 4, "Targeting": 4, "Audiences": 8,
		"LeadForms": 6, "Leads": 3, "Conversions": 4, "Insights": 6, "Reports": 3,
		"Rules": 11, "Results": 5, "Media": 4, "Previews": 2, "Partnership": 3,
		"Accounts": 11, "ReachFrequency": 4, "Studies": 2, "BudgetSchedules": 3, "Google": 9,
	}
	total := 2 // Ads.BulkStatus and Ads.Duplicate
	for name, want := range counts {
		got := v.FieldByName(name).Type().NumMethod()
		if got != want {
			t.Fatalf("client.Ads.%s has %d methods, want %d", name, got, want)
		}
		total += got
	}
	if total != 108 {
		t.Fatalf("expected 108 parity methods, got %d", total)
	}
	if len(AdsWebhookEvents) != 13 {
		t.Fatalf("expected 13 ads webhook events, got %d", len(AdsWebhookEvents))
	}
}

func TestAdsParityRequests(t *testing.T) {
	var requests []parityRequest
	client := parityClient(t, &requests, `{"data":{}}`)
	ctx := context.Background()
	last := func() parityRequest { return requests[len(requests)-1] }

	must := func(_ any, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(client.Ads.Objects.Tree(ctx, 3, &AdTreeParams{From: "2026-10-01", Status: []string{"active", "paused"}, Limit: 10}))
	if r := last(); r.method != http.MethodGet || r.path != "/ads/accounts/3/tree" ||
		r.query != "from=2026-10-01&limit=10&status=active%2Cpaused" {
		t.Fatalf("Objects.Tree sent %s %s?%s", r.method, r.path, r.query)
	}

	must(client.Ads.Objects.Update(ctx, 3, "ad_set", "238", &AdObjectUpdateParams{Name: "Autumn", EndAt: Null}))
	if r := last(); r.method != http.MethodPatch || r.path != "/ads/accounts/3/objects/ad_set/238" ||
		r.rawBody != `{"name":"Autumn","end_at":null}` {
		t.Fatalf("Objects.Update sent %s %s %s", r.method, r.path, r.rawBody)
	}

	must(client.Ads.Objects.SetStatus(ctx, 3, "ad", "77", &AdStatusParams{Status: "paused"}))
	if r := last(); r.method != http.MethodPut || r.path != "/ads/accounts/3/objects/ad/77/status" || r.rawBody != `{"status":"paused"}` {
		t.Fatalf("Objects.SetStatus sent %s %s %s", r.method, r.path, r.rawBody)
	}

	must(client.Ads.Audiences.RemoveMembers(ctx, "aud 1", &AdAudienceMembersParams{
		AdAccountID: 3, Schema: []string{"email"}, Rows: [][]string{{"fake-hash-1"}},
	}))
	if r := last(); r.method != http.MethodDelete || r.path != "/ads/audiences/aud%201/members" ||
		r.rawBody != `{"ad_account_id":3,"schema":["email"],"rows":[["fake-hash-1"]]}` {
		t.Fatalf("Audiences.RemoveMembers sent %s %s %s", r.method, r.path, r.rawBody)
	}

	must(client.Ads.Audiences.Create(ctx, &AdAudienceCreateParams{
		AdAccountID: 3, Name: "Buyers", Kind: "customer_list", CustomerFileSource: "user_provided",
		Schema: []string{"email"}, Rows: [][]string{{"fake-hash-2"}},
	}))
	if r := last(); r.body["customer_file_source"] != "user_provided" || r.body["schema"] == nil || r.body["rows"] == nil {
		t.Fatalf("Audiences.Create sent %s", r.rawBody)
	}

	must(client.Ads.Media.Delete(ctx, "v9", &AdMediaRefParams{AdAccountID: 3, Type: "video"}))
	if r := last(); r.method != http.MethodDelete || r.path != "/ads/media/v9" ||
		r.query != "ad_account_id=3&type=video" || r.rawBody != "" {
		t.Fatalf("Media.Delete sent %s %s?%s %q", r.method, r.path, r.query, r.rawBody)
	}

	must(client.Ads.LeadForms.Create(ctx, &AdLeadFormCreateParams{
		AdAccountID: 3, Name: "Quote", PrivacyPolicyURL: "https://example.com/privacy",
		Questions: []AdLeadFormQuestion{{Type: "email"}}, ThankYou: AdLeadFormThankYou{Title: "Thanks"},
		AcceptTerms: Bool(true),
	}))
	if r := last(); r.path != "/ads/lead-forms" || r.body["accept_terms"] != true {
		t.Fatalf("LeadForms.Create sent %s", r.rawBody)
	}

	must(client.Ads.ReachFrequency.Create(ctx, &AdReachFrequencyParams{
		AdAccountID: 3, Objective: "awareness", Targeting: AdCampaignTargeting{Countries: []string{"NL"}},
		StartAt: "2026-12-01T00:00:00Z", EndAt: "2026-12-14T00:00:00Z", Reach: 200000, FrequencyCap: 2,
	}))
	if r := last(); r.path != "/ads/reach-frequency" || r.rawBody != `{"ad_account_id":3,"objective":"awareness",`+
		`"targeting":{"countries":["NL"]},"start_at":"2026-12-01T00:00:00Z","end_at":"2026-12-14T00:00:00Z",`+
		`"reach":200000,"frequency_cap":2}` {
		t.Fatalf("ReachFrequency.Create sent %s", r.rawBody)
	}

	must(client.Ads.Insights.Query(ctx, &AdInsightsQueryParams{AdAccountID: 3, Level: "ad", Breakdowns: []string{"country"}, From: "2026-10-01"}))
	if r := last(); r.rawBody != `{"ad_account_id":3,"level":"ad","breakdowns":["country"],"from":"2026-10-01"}` {
		t.Fatalf("Insights.Query sent %s", r.rawBody)
	}

	must(client.Ads.Accounts.UpdateSettings(ctx, 3, &AdAccountSettingsParams{SpendCap: Null, ConfirmBudgetAmount: "remove"}))
	if r := last(); r.path != "/ads/accounts/3/settings" || r.rawBody != `{"spend_cap":null,"confirm_budget_amount":"remove"}` {
		t.Fatalf("Accounts.UpdateSettings sent %s %s", r.path, r.rawBody)
	}

	must(client.Ads.Google.AdSchedule(ctx, 3, "987"))
	if r := last(); r.path != "/ads/google/ad-schedule" || r.query != "ad_account_id=3&external_campaign_id=987" {
		t.Fatalf("Google.AdSchedule sent %s?%s", r.path, r.query)
	}

	must(client.Ads.Update(ctx, 7, &AdCampaignUpdateParams{
		Name: "Leads", Objective: "lead_generation",
		Settings: &AdNetworkSettings{LeadFormID: "lf-1", Optimization: &AdOptimizationSettings{
			Bidding: &AdBiddingSetting{Strategy: "cost_cap", Amount: 4},
		}},
	}))
	if r := last(); r.method != http.MethodPatch || r.path != "/ads/campaigns/7" || !reflect.DeepEqual(r.body["settings"], map[string]any{
		"optimization": map[string]any{"bidding": map[string]any{"strategy": "cost_cap", "amount": float64(4)}},
		"lead_form_id": "lf-1",
	}) {
		t.Fatalf("Ads.Update sent %s %s %s", r.method, r.path, r.rawBody)
	}

	draft := testAdDraftParams()
	draft.Platforms = []AdDraftPlatform{{Platform: "meta", Settings: &AdNetworkSettings{LeadFormID: "lf-1"}}}
	draft.Targeting.Networks = map[string]AdNetworkTargeting{
		"meta": {Include: map[string][]AdTargetingItem{"interest": {{ID: "6003", Name: "Running"}}}},
	}
	must(client.Ads.Drafts.Create(ctx, draft))
	r := last()
	platforms, _ := r.body["platforms"].([]any)
	targeting, _ := r.body["targeting"].(map[string]any)
	if len(platforms) != 1 || platforms[0].(map[string]any)["settings"] == nil || targeting["networks"] == nil {
		t.Fatalf("Drafts.Create did not send platform settings and network targeting: %s", r.rawBody)
	}
}

// Every parity call that can start or raise spend is sent exactly once after
// a 5xx: the first attempt may have gone through.
func TestAdsParityMoneyCallsAreNotRetriedAfter5xx(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"error":{"code":"bad_gateway","message":"upstream"}}`)
	}), WithMaxRetries(2))
	ctx := context.Background()
	call := func(_ any, err error) error { return err }

	cases := map[string]func() error{
		"Objects.Update": func() error {
			return call(client.Ads.Objects.Update(ctx, 3, "ad_set", "238", &AdObjectUpdateParams{
				Budget: &AdBudget{Amount: 40, Currency: "EUR"}, ConfirmBudgetAmount: 40,
			}))
		},
		"Objects.SetStatus": func() error {
			return call(client.Ads.Objects.SetStatus(ctx, 3, "campaign", "120", &AdStatusParams{Status: "active", ConfirmBudgetAmount: 20}))
		},
		"BulkStatus": func() error {
			return call(client.Ads.BulkStatus(ctx, &AdBulkStatusParams{CampaignIDs: []AdID{7, 8}, Status: "active", ConfirmBudgetAmount: 25}))
		},
		"CampaignAds.SetStatus": func() error {
			return call(client.Ads.CampaignAds.SetStatus(ctx, 7, 51, &AdStatusParams{Status: "active", ConfirmBudgetAmount: 25}))
		},
		"Rules.Create": func() error {
			return call(client.Ads.Rules.Create(ctx, &AdRuleParams{
				Name:                  "Scale winners",
				Conditions:            []AdRuleCondition{{Metric: "ctr", Op: "gt", Value: 2}},
				Action:                &AdRuleAction{Type: "adjust_budget", Percent: 20, MaxDailyBudget: 50},
				ConfirmMaxDailyBudget: 50,
			}))
		},
		"Rules.Update": func() error { return call(client.Ads.Rules.Update(ctx, 4, &AdRuleParams{Enabled: Bool(true)})) },
		"Rules.Undo":   func() error { return call(client.Ads.Rules.Undo(ctx, 9, 25)) },
		"Accounts.UpdateSettings": func() error {
			return call(client.Ads.Accounts.UpdateSettings(ctx, 3, &AdAccountSettingsParams{SpendCap: 500, ConfirmBudgetAmount: 500}))
		},
		"ReachFrequency.Reserve": func() error {
			return call(client.Ads.ReachFrequency.Reserve(ctx, "rf-1", &AdReachFrequencyReserveParams{AdAccountID: 3, ConfirmBudgetAmount: 1000}))
		},
		"BudgetSchedules.Create": func() error {
			return call(client.Ads.BudgetSchedules.Create(ctx, 7, &AdBudgetScheduleParams{
				StartAt: "2026-11-27T00:00:00Z", EndAt: "2026-11-30T00:00:00Z",
				BudgetIncrease: AdBudgetIncrease{Type: "absolute", Value: 30}, ConfirmBudgetAmount: 30,
			}))
		},
	}
	for name, fn := range cases {
		calls.Store(0)
		assertErrorAs[*ServerError](t, fn())
		if got := calls.Load(); got != 1 {
			t.Fatalf("%s must be sent once after a 502, got %d attempts", name, got)
		}
	}

	// A parity read keeps the normal retries.
	calls.Store(0)
	if _, err := client.Ads.Objects.Tree(ctx, 3, nil); err == nil {
		t.Fatal("expected an error")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("Objects.Tree (a read) should be tried 3 times, got %d", got)
	}
}

func TestAdsParityCapabilitiesAndTreeDecode(t *testing.T) {
	var requests []parityRequest
	client := parityClient(t, &requests, `{"data":{"networks":[{"key":"meta","label":"Meta","runs_on":"","identity":"",
		"goals":{"lead_generation":{"ok":true,"apiOnly":true,"needs":["lead_form"]}},"media":{},"text":{},"ctas":[],
		"no_cta":false,"extra_fields":[],"min_daily_budget":{"default":1},"review":"",
		"bid_strategies":["lowest_cost","cost_cap"],"settings":{"lead_form_id":"Meta instant form id"},
		"ad_fields":{"media_ref":"from the library"}}],"goals":[],"ctas":[],"create_networks":["meta"],
		"features":{"meta":["tree","audiences"]},"feature_labels":{"tree":"Campaigns"},
		"targeting_types":{"meta":["interest","location"]},"api_goals":["lead_generation","conversions","messages","calls"]}}`)
	caps, err := client.Ads.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	meta := caps.Data.Networks[0]
	if !meta.Goals["lead_generation"].APIOnly || meta.Goals["lead_generation"].Needs[0] != "lead_form" ||
		len(meta.BidStrategies) != 2 || meta.Settings["lead_form_id"] == "" || meta.AdFields["media_ref"] == "" ||
		caps.Data.Features["meta"][1] != "audiences" || caps.Data.TargetingTypes["meta"][0] != "interest" ||
		len(caps.Data.APIGoals) != 4 || caps.Data.FeatureLabels["tree"] != "Campaigns" {
		t.Fatalf("unexpected capabilities: %+v", caps.Data)
	}

	var treeRequests []parityRequest
	treeClient := parityClient(t, &treeRequests, `{"data":{"ad_account_id":"3","platform":"meta","currency":"EUR",
		"from":"2026-10-01","to":"2026-10-31","next_cursor":null,"campaigns":[{"level":"campaign","external_id":"120",
		"name":"Outside","status":"active","budget":{"type":"daily","amount":20,"currency":"EUR"},"objective":null,
		"start_at":null,"end_at":null,"metrics":{"spend":12.5,"impressions":4000,"reach":null},"source":"external",
		"campaign_id":null,"children":[{"level":"ad_set","external_id":"238","name":"Set","status":"paused",
		"budget":null,"objective":null,"start_at":null,"end_at":null,"metrics":null}]}]}}`)
	tree, err := treeClient.Ads.Objects.Tree(context.Background(), 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := tree.Data.Campaigns[0]
	if tree.Data.AdAccountID != 3 || c.Source != "external" || c.Metrics == nil || *c.Metrics.Spend != 12.5 ||
		c.Metrics.Reach != nil || len(c.Children) != 1 || c.Children[0].Budget != nil || treeRequests[0].query != "" {
		t.Fatalf("unexpected tree: %+v", tree.Data)
	}
}
