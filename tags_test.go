package omnisocials

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type recordedRequest struct {
	method string
	path   string
	query  string
	body   map[string]any
}

func recordRequests(t *testing.T, reply string, status int) (*Client, *[]recordedRequest) {
	t.Helper()
	var got []recordedRequest
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery}
		data, _ := io.ReadAll(r.Body)
		if len(data) > 0 {
			if err := json.Unmarshal(data, &req.body); err != nil {
				t.Errorf("body is not a JSON object: %s", data)
			}
		}
		got = append(got, req)
		w.WriteHeader(status)
		fmt.Fprint(w, reply)
	}), WithMaxRetries(0))
	return client, &got
}

func TestTagsListCreateUpdateDelete(t *testing.T) {
	ctx := context.Background()
	tag := `{"id":12,"name":"Question","color":"blue","created_at":"2026-11-02T10:00:00Z"}`

	client, got := recordRequests(t, `{"data":[`+tag+`]}`, http.StatusOK)
	list, err := client.Tags.List(ctx)
	if err != nil {
		t.Fatalf("Tags.List: %v", err)
	}
	if (*got)[0].method != http.MethodGet || (*got)[0].path != "/tags" {
		t.Fatalf("unexpected request: %+v", (*got)[0])
	}
	if len(list.Data) != 1 || list.Data[0].ID != 12 || list.Data[0].Name != "Question" || list.Data[0].Color != "blue" {
		t.Fatalf("unexpected tags: %+v", list.Data)
	}

	client, got = recordRequests(t, `{"data":`+tag+`}`, http.StatusCreated)
	created, err := client.Tags.Create(ctx, &TagCreateParams{Name: "Question"})
	if err != nil {
		t.Fatalf("Tags.Create: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodPost || req.path != "/tags" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.body["name"] != "Question" {
		t.Fatalf("expected name in body, got %v", req.body)
	}
	if _, ok := req.body["color"]; ok {
		t.Fatalf("an empty Color must be left out, got %v", req.body)
	}
	if created.Data.ID != 12 {
		t.Fatalf("unexpected tag: %+v", created.Data)
	}

	client, got = recordRequests(t, `{"data":`+tag+`}`, http.StatusOK)
	if _, err := client.Tags.Update(ctx, 12, &TagUpdateParams{Color: "green"}); err != nil {
		t.Fatalf("Tags.Update: %v", err)
	}
	req = (*got)[0]
	if req.method != http.MethodPatch || req.path != "/tags/12" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.body["color"] != "green" || req.body["name"] != nil {
		t.Fatalf("expected only color in body, got %v", req.body)
	}

	client, got = recordRequests(t, `{"data":{"tag":`+tag+`,"posts_untagged":7}}`, http.StatusOK)
	deleted, err := client.Tags.Delete(ctx, 12)
	if err != nil {
		t.Fatalf("Tags.Delete: %v", err)
	}
	if (*got)[0].method != http.MethodDelete || (*got)[0].path != "/tags/12" {
		t.Fatalf("unexpected request: %+v", (*got)[0])
	}
	if deleted.Data.PostsUntagged != 7 || deleted.Data.Tag.Name != "Question" {
		t.Fatalf("unexpected delete result: %+v", deleted.Data)
	}
}

func TestTagsApplyTakesIdsAndNames(t *testing.T) {
	reply := `{"data":{"updated":1,"posts":[{"id":"1024","tags":[{"id":12,"name":"Question","color":"blue"}]}],"not_found":["9"]}}`
	client, got := recordRequests(t, reply, http.StatusOK)

	res, err := client.Tags.Apply(context.Background(), &TagApplyParams{
		PostIDs: []string{"1024", "9"},
		Add:     []any{12, "Talking head"},
		Remove:  []any{"Old"},
	})
	if err != nil {
		t.Fatalf("Tags.Apply: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodPost || req.path != "/tags/apply" {
		t.Fatalf("unexpected request: %+v", req)
	}
	add, _ := req.body["add"].([]any)
	if len(add) != 2 || add[0] != float64(12) || add[1] != "Talking head" {
		t.Fatalf("expected add [12, \"Talking head\"], got %v", req.body["add"])
	}
	if _, ok := req.body["set"]; ok {
		t.Fatalf("a nil Set must be left out, got %v", req.body)
	}
	if res.Data.Updated != 1 || res.Data.Posts[0].Tags[0].ID != 12 || res.Data.NotFound[0] != "9" {
		t.Fatalf("unexpected apply result: %+v", res.Data)
	}

	// An empty Set removes every tag of the posts.
	client, got = recordRequests(t, reply, http.StatusOK)
	if _, err := client.Tags.Apply(context.Background(), &TagApplyParams{PostIDs: []string{"1024"}, Set: []any{}}); err != nil {
		t.Fatalf("Tags.Apply: %v", err)
	}
	set, ok := (*got)[0].body["set"].([]any)
	if !ok || len(set) != 0 {
		t.Fatalf("expected set [], got %v", (*got)[0].body)
	}
}

func TestTagErrorCarriesCode(t *testing.T) {
	client, _ := recordRequests(t,
		`{"error":{"code":"tag_exists","message":"A tag with this name exists.","tag":{"id":12,"name":"Question","color":"blue"}}}`,
		http.StatusConflict)
	_, err := client.Tags.Create(context.Background(), &TagCreateParams{Name: "question"})
	apiErr := assertErrorAs[*APIError](t, err)
	if apiErr.Status != http.StatusConflict || apiErr.Code != "tag_exists" {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestAnalyticsTagsQueryAndDecode(t *testing.T) {
	reply := `{"data":{
		"period":{"start_date":"2026-08-01T00:00:00.000Z","end_date":"2026-10-31T23:59:59.999Z"},
		"platform":"tiktok","min_posts":3,
		"tags":[{"tag":{"id":12,"name":"Question","color":"blue"},"posts":6,"measured":5,
			"median_impressions":1200,"engagement_rate":5.4,"vs_normal":0.38,"best_channel":"tiktok","enough_posts":true}],
		"untagged":{"posts":4,"measured":4,"median_impressions":null,"engagement_rate":null,"vs_normal":null,"best_channel":null,"enough_posts":true},
		"normals":{"tiktok":870,"instagram":null}}}`
	client, got := recordRequests(t, reply, http.StatusOK)

	res, err := client.Analytics.Tags(context.Background(), &TagAnalyticsParams{
		StartDate: "2026-08-01", EndDate: "2026-10-31", Platform: "tiktok", MinPosts: 3,
	})
	if err != nil {
		t.Fatalf("Analytics.Tags: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodGet || req.path != "/analytics/tags" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.query != "end_date=2026-10-31&min_posts=3&platform=tiktok&start_date=2026-08-01" {
		t.Fatalf("unexpected query %q", req.query)
	}
	data := res.Data
	if len(data.Tags) != 1 || data.Tags[0].Tag.Name != "Question" || data.Tags[0].Posts != 6 {
		t.Fatalf("unexpected tags: %+v", data.Tags)
	}
	if data.Tags[0].VsNormal == nil || *data.Tags[0].VsNormal != 0.38 || !data.Tags[0].EnoughPosts {
		t.Fatalf("unexpected comparison: %+v", data.Tags[0])
	}
	if data.Untagged.VsNormal != nil || data.Untagged.Posts != 4 {
		t.Fatalf("unexpected untagged: %+v", data.Untagged)
	}
	if data.Normals["tiktok"] == nil || *data.Normals["tiktok"] != 870 || data.Normals["instagram"] != nil {
		t.Fatalf("unexpected normals: %+v", data.Normals)
	}
	if data.Platform == nil || *data.Platform != "tiktok" || data.MinPosts != 3 {
		t.Fatalf("unexpected header fields: %+v", data)
	}
}

func TestTagFilterOnPostsListAndOverview(t *testing.T) {
	client, got := recordRequests(t, `{"data":[{"id":"1","status":"published","content":"hi",
		"tags":[{"id":12,"name":"Question","color":"blue"}],"trial_reel":{"strategy":"SS_PERFORMANCE"}}]}`, http.StatusOK)
	posts, err := client.Posts.List(context.Background(), &PostListParams{Tag: "Question,14"})
	if err != nil {
		t.Fatalf("Posts.List: %v", err)
	}
	if (*got)[0].query != "tag=Question%2C14" {
		t.Fatalf("unexpected query %q", (*got)[0].query)
	}
	p := posts.Data[0]
	if len(p.Tags) != 1 || p.Tags[0].ID != 12 || p.TrialReel == nil || p.TrialReel.Strategy != "SS_PERFORMANCE" {
		t.Fatalf("unexpected post: %+v", p)
	}

	client, got = recordRequests(t, `{"data":{"total_posts":3}}`, http.StatusOK)
	if _, err := client.Analytics.Overview(context.Background(), &AnalyticsOverviewParams{Period: "30d", Tag: "12"}); err != nil {
		t.Fatalf("Analytics.Overview: %v", err)
	}
	if (*got)[0].query != "period=30d&tag=12" {
		t.Fatalf("unexpected query %q", (*got)[0].query)
	}

	client, _ = recordRequests(t, `{"data":[{"post_id":"1","platforms":{},"trial_reel":null,"tags":[{"id":12,"name":"Question","color":"blue"}]}],"count":1}`, http.StatusOK)
	batch, err := client.Analytics.Posts(context.Background(), []string{"1"})
	if err != nil {
		t.Fatalf("Analytics.Posts: %v", err)
	}
	if batch.Data[0].TrialReel != nil || len(batch.Data[0].Tags) != 1 {
		t.Fatalf("unexpected batch entry: %+v", batch.Data[0])
	}
}

func TestPostTagsSerialize(t *testing.T) {
	create, err := json.Marshal(PostCreateParams{Content: "hi", Tags: []any{"Question", 14}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"tags":["Question",14]`; !strings.Contains(string(create), want) {
		t.Fatalf("expected %s in %s", want, create)
	}
	none, _ := json.Marshal(PostCreateParams{Content: "hi"})
	if strings.Contains(string(none), `"tags"`) {
		t.Fatalf("nil Tags must be left out: %s", none)
	}

	cleared, _ := json.Marshal(PostUpdateParams{Tags: Null})
	if string(cleared) != `{"tags":null}` {
		t.Fatalf("Null must send tags null, got %s", cleared)
	}
	replaced, _ := json.Marshal(PostUpdateParams{Tags: []any{12}})
	if string(replaced) != `{"tags":[12]}` {
		t.Fatalf("unexpected update body %s", replaced)
	}
	kept, _ := json.Marshal(PostUpdateParams{Content: "x"})
	if strings.Contains(string(kept), `"tags"`) {
		t.Fatalf("nil Tags must be left out: %s", kept)
	}
}

func TestTagsServiceMethodsExist(t *testing.T) {
	client, err := NewClient(WithAPIKey("omsk_test_key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	methods := []any{
		client.Tags.List, client.Tags.Create, client.Tags.Update,
		client.Tags.Delete, client.Tags.Apply, client.Analytics.Tags,
	}
	for i, m := range methods {
		if m == nil {
			t.Fatalf("method %d is nil", i)
		}
	}
}
