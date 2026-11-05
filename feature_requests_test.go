package omnisocials

import (
	"context"
	"net/http"
	"testing"
)

const testFeatureRequest = `{"id":412,"title":"Post to a topic of a Telegram group",
	"summary":"Post our daily update to the Announcements topic of our Telegram group.",
	"status":"open","team_answer":null,"team_replies":[{"message":"On the list.","created_at":"2026-11-05T09:00:00.000Z"}],
	"votes_count":2,"limit_id":"telegram.forum_topics","platform":"telegram","area":"posts","source":"api",
	"created_at":"2026-11-04T10:00:00.000Z","updated_at":"2026-11-05T09:00:00.000Z","completed_at":null}`

const testPlatformLimit = `{"id":"tiktok.delete","kind":"platform","kind_label":"The platform does not allow it",
	"area":"posts","platforms":["tiktok"],"title":"Delete a TikTok video",
	"says":"TikTok's API cannot delete a video.","instead":"Delete the video in the TikTok app.","requestable":false}`

func TestLimitsListQueryAndDecode(t *testing.T) {
	client, got := recordRequests(t,
		`{"data":[`+testPlatformLimit+`],"kinds":{"platform":"The platform does not allow it"}}`, http.StatusOK)
	res, err := client.Limits.List(context.Background(), &LimitListParams{Platform: "tiktok", Area: "posts"})
	if err != nil {
		t.Fatalf("Limits.List: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodGet || req.path != "/limits" || req.query != "area=posts&platform=tiktok" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if len(res.Data) != 1 {
		t.Fatalf("unexpected limits: %+v", res.Data)
	}
	l := res.Data[0]
	if l.ID != "tiktok.delete" || l.Kind != "platform" || l.Requestable || l.Platforms[0] != "tiktok" {
		t.Fatalf("unexpected limit: %+v", l)
	}
	if l.Instead == nil || *l.Instead != "Delete the video in the TikTok app." {
		t.Fatalf("unexpected instead: %v", l.Instead)
	}
	if res.Kinds["platform"] != "The platform does not allow it" {
		t.Fatalf("unexpected kinds: %+v", res.Kinds)
	}

	client, got = recordRequests(t, `{"data":[],"kinds":{}}`, http.StatusOK)
	if _, err := client.Limits.List(context.Background(), nil); err != nil {
		t.Fatalf("Limits.List(nil): %v", err)
	}
	if (*got)[0].query != "" {
		t.Fatalf("nil params must send no query, got %q", (*got)[0].query)
	}
}

func TestFeatureRequestsCreateReturnsTheOutcome(t *testing.T) {
	client, got := recordRequests(t, `{"outcome":"voted","data":`+testFeatureRequest+`}`, http.StatusCreated)
	res, err := client.FeatureRequests.Create(context.Background(), &FeatureRequestCreateParams{
		Summary:  "Post our daily update to the Announcements topic of our Telegram group.",
		Platform: "telegram",
		LimitID:  "telegram.forum_topics",
	})
	if err != nil {
		t.Fatalf("FeatureRequests.Create: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodPost || req.path != "/feature-requests" {
		t.Fatalf("unexpected request: %+v", req)
	}
	if req.body["summary"] == nil || req.body["platform"] != "telegram" || req.body["limit_id"] != "telegram.forum_topics" {
		t.Fatalf("unexpected body: %v", req.body)
	}
	for _, key := range []string{"title", "area", "source"} {
		if _, ok := req.body[key]; ok {
			t.Fatalf("%s must be left out, got %v", key, req.body)
		}
	}
	if res.Outcome != FeatureRequestVoted || res.Data.ID != 412 || res.Data.VotesCount != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Data.LimitID == nil || *res.Data.LimitID != "telegram.forum_topics" || res.Data.TeamAnswer != nil {
		t.Fatalf("unexpected request fields: %+v", res.Data)
	}
	if len(res.Data.TeamReplies) != 1 || res.Data.TeamReplies[0].Message != "On the list." {
		t.Fatalf("unexpected team replies: %+v", res.Data.TeamReplies)
	}

	// A 200 already_requested is a success that keeps its outcome.
	client, _ = recordRequests(t, `{"outcome":"already_requested","data":`+testFeatureRequest+`}`, http.StatusOK)
	again, err := client.FeatureRequests.Create(context.Background(), &FeatureRequestCreateParams{Summary: "Post to a Telegram topic, please."})
	if err != nil {
		t.Fatalf("FeatureRequests.Create: %v", err)
	}
	if again.Outcome != FeatureRequestAlreadyRequested {
		t.Fatalf("unexpected outcome %q", again.Outcome)
	}
}

func TestFeatureRequestNotRequestableKeepsTheLimit(t *testing.T) {
	client, _ := recordRequests(t,
		`{"error":{"code":"not_requestable","message":"TikTok's API cannot delete a video. A request cannot change this, so nothing was sent.","limit":`+testPlatformLimit+`}}`,
		http.StatusConflict)
	_, err := client.FeatureRequests.Create(context.Background(), &FeatureRequestCreateParams{
		Summary: "Delete my TikTok videos from OmniSocials.", LimitID: "tiktok.delete",
	})
	apiErr := assertErrorAs[*APIError](t, err)
	if apiErr.Status != http.StatusConflict || apiErr.Code != "not_requestable" {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
	body, _ := apiErr.Body.(map[string]any)
	errObj, _ := body["error"].(map[string]any)
	limit, _ := errObj["limit"].(map[string]any)
	if limit["id"] != "tiktok.delete" {
		t.Fatalf("expected the limit in the error body, got %v", apiErr.Body)
	}
}

func TestFeatureRequestsListAndGet(t *testing.T) {
	client, got := recordRequests(t, `{"data":[`+testFeatureRequest+`]}`, http.StatusOK)
	list, err := client.FeatureRequests.List(context.Background())
	if err != nil {
		t.Fatalf("FeatureRequests.List: %v", err)
	}
	if (*got)[0].method != http.MethodGet || (*got)[0].path != "/feature-requests" {
		t.Fatalf("unexpected request: %+v", (*got)[0])
	}
	if len(list.Data) != 1 || list.Data[0].Status != "open" || list.Data[0].Source != "api" {
		t.Fatalf("unexpected list: %+v", list.Data)
	}

	client, got = recordRequests(t, `{"data":`+testFeatureRequest+`}`, http.StatusOK)
	one, err := client.FeatureRequests.Get(context.Background(), 412)
	if err != nil {
		t.Fatalf("FeatureRequests.Get: %v", err)
	}
	if (*got)[0].path != "/feature-requests/412" {
		t.Fatalf("unexpected path %q", (*got)[0].path)
	}
	if one.Data.Title != "Post to a topic of a Telegram group" || one.Data.CompletedAt != nil {
		t.Fatalf("unexpected request: %+v", one.Data)
	}

	client, _ = recordRequests(t, `{"error":{"code":"not_found","message":"Feature request not found."}}`, http.StatusNotFound)
	_, err = client.FeatureRequests.Get(context.Background(), 9)
	notFound := assertErrorAs[*NotFoundError](t, err)
	if notFound.Code != "not_found" {
		t.Fatalf("unexpected error: %+v", notFound)
	}
}

func TestFeatureRequestServicesExist(t *testing.T) {
	client, err := NewClient(WithAPIKey("omsk_test_key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	methods := []any{
		client.Limits.List, client.FeatureRequests.Create,
		client.FeatureRequests.List, client.FeatureRequests.Get,
	}
	for i, m := range methods {
		if m == nil {
			t.Fatalf("method %d is nil", i)
		}
	}
}
