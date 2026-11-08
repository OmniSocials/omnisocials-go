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

const testXAccount = `{"id":"1234_x","platform":"x","username":"omnisocials","display_name":"OmniSocials",
	"profile_picture":null,"content_types":["post"],"status":"active","needs_reconnect":false,"connected_at":null,
	"x_statistics":{"posts":"light","followers":"daily","post_schedule":null,"source":"account","billing":"credits",
	"state":"paused","paused_reason":"insufficient_credits"}}`

// rawBodies records the raw request bodies, so a test can tell null from a
// missing key.
func rawBodies(t *testing.T, reply string) (*Client, *[]string, *[]string) {
	t.Helper()
	var bodies, paths []string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(data))
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		fmt.Fprint(w, reply)
	}), WithMaxRetries(0))
	return client, &bodies, &paths
}

func TestAccountsUpdateSendsOnlyTheGivenXStatisticsKeys(t *testing.T) {
	ctx := context.Background()
	client, bodies, paths := rawBodies(t, `{"data":`+testXAccount+`}`)

	res, err := client.Accounts.Update(ctx, "1234_x", &AccountUpdateParams{
		XStatistics: &XStatisticsInput{Posts: XStatisticsPostsLight, Followers: XStatisticsFollowersDaily},
	})
	if err != nil {
		t.Fatalf("Accounts.Update: %v", err)
	}
	if (*paths)[0] != "PATCH /accounts/1234_x" || (*bodies)[0] != `{"x_statistics":{"posts":"light","followers":"daily"}}` {
		t.Fatalf("unexpected request: %s %s", (*paths)[0], (*bodies)[0])
	}
	x := res.Data.XStatistics
	if x == nil || x.Posts != "light" || x.State != "paused" || x.PausedReason == nil || *x.PausedReason != "insufficient_credits" ||
		x.PostSchedule != nil || x.Billing != "credits" || x.Source != "account" {
		t.Fatalf("unexpected x_statistics: %+v", x)
	}

	cases := []struct {
		params *AccountUpdateParams
		want   string
	}{
		{&AccountUpdateParams{XStatistics: &XStatisticsInput{Followers: XStatisticsOff}}, `{"x_statistics":{"followers":"off"}}`},
		{&AccountUpdateParams{XStatistics: &XStatisticsInput{
			Posts:        XStatisticsPostsCustom,
			PostSchedule: &XStatisticsPostSchedule{At: []string{"1h", "1d"}, ThenEvery: "7d", Until: "90d"},
		}}, `{"x_statistics":{"posts":"custom","post_schedule":{"at":["1h","1d"],"then_every":"7d","until":"90d"}}}`},
		{&AccountUpdateParams{}, `{"x_statistics":null}`},
		{nil, `{}`},
	}
	for i, tc := range cases {
		if _, err := client.Accounts.Update(ctx, "1234_x", tc.params); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if got := (*bodies)[i+1]; got != tc.want {
			t.Fatalf("case %d: body %s, want %s", i, got, tc.want)
		}
	}
}

func TestAccountsListDecodesXStatistics(t *testing.T) {
	client, got := recordRequests(t, `{"data":[`+testXAccount+`,{"id":"1234_instagram","platform":"instagram","status":"active"}]}`, http.StatusOK)
	res, err := client.Accounts.List(context.Background())
	if err != nil {
		t.Fatalf("Accounts.List: %v", err)
	}
	if (*got)[0].path != "/accounts" {
		t.Fatalf("unexpected request: %+v", (*got)[0])
	}
	if res.Data[0].XStatistics == nil || res.Data[0].XStatistics.Followers != "daily" || res.Data[1].XStatistics != nil {
		t.Fatalf("unexpected accounts: %+v", res.Data)
	}
}

func TestAnalyticsRefreshPost(t *testing.T) {
	client, got := recordRequests(t, `{"data":{"post_id":9123,"tweets":[{"id":"1859","metrics":{"views":3412},
		"read_at":"2027-01-08T09:12:04Z"}],"credits_used":0.5,"read_at":"2027-01-08T09:12:04Z"}}`, http.StatusOK)
	res, err := client.Analytics.RefreshPost(context.Background(), "9123")
	if err != nil {
		t.Fatalf("Analytics.RefreshPost: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodPost || req.path != "/analytics/posts/9123/refresh" || req.body != nil {
		t.Fatalf("unexpected request: %+v", req)
	}
	if res.Data.PostID.String() != "9123" || res.Data.CreditsUsed != 0.5 || len(res.Data.Tweets) != 1 ||
		res.Data.Tweets[0].ID != "1859" || res.Data.Tweets[0].Metrics["views"] != float64(3412) {
		t.Fatalf("unexpected result: %+v", res.Data)
	}
}

func TestAnalyticsRefreshPostIsNotRetriedAfter5xxOrAConnectionError(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"error":{"code":"refresh_failed","message":"X did not answer. Nothing was charged."}}`)
	}), WithMaxRetries(2))
	_, err := client.Analytics.RefreshPost(context.Background(), "9123")
	if apiErr := assertErrorAs[*ServerError](t, err); apiErr.Code != "refresh_failed" {
		t.Fatalf("unexpected code: %q", apiErr.Code)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("a refresh must be sent once after a 502, got %d attempts", got)
	}

	calls.Store(0)
	dropping := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	}), WithMaxRetries(2))
	_, err = dropping.Analytics.RefreshPost(context.Background(), "9123")
	assertErrorAs[*ConnectionError](t, err)
	if got := calls.Load(); got != 1 {
		t.Fatalf("a refresh must be sent once after a dropped connection, got %d attempts", got)
	}
}

func TestAnalyticsRefreshPostIsRetriedAfter429(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"rate_limit_exceeded","message":"slow down"}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"post_id":"9123","tweets":[],"credits_used":0,"read_at":"2027-01-08T09:12:04Z"}}`)
	}), WithMaxRetries(2))
	res, err := client.Analytics.RefreshPost(context.Background(), "9123")
	if err != nil {
		t.Fatalf("expected the 429 to be retried, got: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
	if res.Data.PostID.String() != "9123" {
		t.Fatalf("a string post_id must decode too, got %q", res.Data.PostID)
	}
}

func TestCreditsGet(t *testing.T) {
	client, got := recordRequests(t, `{"data":{"balance":412,"auto_refill":{"enabled":false},
		"this_month":{"x_statistics":31,"x_url_post":40},
		"x_statistics":{"state":"active","paused_at":null,"charging":{"active":true,"from":"2027-01-01T00:00:00.000Z"},
		"prices":{"post_read":0.5,"media_read":0.5,"follower_read":1},"total_credits_month":38,
		"this_month":{"post_reads":40,"media_reads":2,"follower_reads":8,"refreshes":1,"credits":31},
		"accounts":[{"account_id":"1234_x","workspace_id":1234,"handle":"@omnisocials","posts":"full","followers":"daily",
		"tweets_30d":12,"credits_month":38}]}}}`, http.StatusOK)
	res, err := client.Credits.Get(context.Background())
	if err != nil {
		t.Fatalf("Credits.Get: %v", err)
	}
	if (*got)[0].method != http.MethodGet || (*got)[0].path != "/credits" {
		t.Fatalf("unexpected request: %+v", (*got)[0])
	}
	d := res.Data
	x := d.XStatistics
	if d.Balance != 412 || d.AutoRefill.Enabled || d.ThisMonth["x_url_post"] != 40 || x.State != "active" || x.PausedAt != nil ||
		!x.Charging.Active || x.Charging.From == nil || x.Prices.FollowerRead != 1 || x.Prices.PostRead != 0.5 ||
		x.TotalCreditsMonth != 38 || x.ThisMonth.Refreshes != 1 || x.ThisMonth.Credits != 31 {
		t.Fatalf("unexpected credits: %+v", d)
	}
	a := x.Accounts[0]
	if a.AccountID != "1234_x" || a.WorkspaceID != 1234 || a.Handle == nil || *a.Handle != "@omnisocials" ||
		a.Tweets30d != 12 || a.CreditsMonth != 38 {
		t.Fatalf("unexpected account: %+v", a)
	}
}

func TestXStatisticsWebhookEvents(t *testing.T) {
	if !reflect.DeepEqual(XStatisticsWebhookEvents, []string{"x_statistics.paused", "x_statistics.resumed"}) {
		t.Fatalf("unexpected events: %v", XStatisticsWebhookEvents)
	}
	payload := []byte(`{"id":"evt_9","type":"x_statistics.paused","created_at":"2027-01-08T09:12:04Z",
		"data":{"workspace_id":1234,"account_id":"1234_x","reason":"insufficient_credits"}}`)
	var event struct {
		Type string               `json:"type"`
		Data XStatisticsEventData `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if event.Type != "x_statistics.paused" || event.Data.WorkspaceID.String() != "1234" ||
		event.Data.AccountID != "1234_x" || event.Data.Reason != "insufficient_credits" {
		t.Fatalf("unexpected event: %+v", event)
	}
}
