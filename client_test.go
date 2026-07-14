package omnisocials

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler, opts ...Option) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	base := []Option{WithAPIKey("omsk_test_key"), WithBaseURL(server.URL)}
	client, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func assertErrorAs[T error](t *testing.T, err error) T {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("expected error of type %T, got %T: %v", target, err, err)
	}
	return target
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"rate_limit_exceeded","message":"Too many requests."}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"id":"1","status":"draft","content":"hi"}}`)
	}), WithMaxRetries(2))

	res, err := client.Posts.Get(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if res.Data.ID != "1" || res.Data.Status != "draft" {
		t.Fatalf("unexpected post: %+v", res.Data)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (1 failure + 1 retry), got %d", got)
	}
}

func TestRetryOn500ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":{"code":"internal_error","message":"boom"}}`)
			return
		}
		fmt.Fprint(w, `{"status":"ok","version":"1.0.0","timestamp":"2026-07-14T00:00:00Z"}`)
	}), WithMaxRetries(1))

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("unexpected health: %+v", health)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestNoRetryOn404AndErrorMapping(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"not_found","message":"Post not found."}}`)
	}), WithMaxRetries(2))

	_, err := client.Posts.Get(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected a 404 error")
	}

	notFound := assertErrorAs[*NotFoundError](t, err)
	if notFound.Status != 404 || notFound.Code != "not_found" || notFound.Message != "Post not found." {
		t.Fatalf("unexpected NotFoundError fields: %+v", notFound.APIError)
	}

	// The typed wrapper also matches the base *APIError via errors.As.
	apiErr := assertErrorAs[*APIError](t, err)
	if apiErr.Status != 404 {
		t.Fatalf("expected base APIError status 404, got %d", apiErr.Status)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("404 must not be retried; got %d attempts", got)
	}
}

func TestErrorMappingByStatus(t *testing.T) {
	serve := func(status int, body string) *Client {
		return newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, body)
		}), WithMaxRetries(0))
	}

	t.Run("400 ValidationError", func(t *testing.T) {
		_, err := serve(400, `{"error":{"code":"validation_error","message":"bad"}}`).Accounts.List(context.Background())
		e := assertErrorAs[*ValidationError](t, err)
		if e.Code != "validation_error" {
			t.Fatalf("unexpected code %q", e.Code)
		}
	})
	t.Run("401 AuthenticationError", func(t *testing.T) {
		_, err := serve(401, `{"error":{"code":"invalid_api_key","message":"bad key"}}`).Accounts.List(context.Background())
		assertErrorAs[*AuthenticationError](t, err)
	})
	t.Run("403 PermissionDeniedError", func(t *testing.T) {
		_, err := serve(403, `{"error":{"code":"insufficient_scope","message":"missing scope"}}`).Accounts.List(context.Background())
		assertErrorAs[*PermissionDeniedError](t, err)
	})
	t.Run("422 ValidationError", func(t *testing.T) {
		_, err := serve(422, `{"error":{"code":"validation_error","message":"bad"}}`).Accounts.List(context.Background())
		assertErrorAs[*ValidationError](t, err)
	})
	t.Run("429 RateLimitError with RetryAfter", func(t *testing.T) {
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"rate_limit_exceeded","message":"slow down"}}`)
		}), WithMaxRetries(0))
		_, err := client.Accounts.List(context.Background())
		e := assertErrorAs[*RateLimitError](t, err)
		if e.RetryAfter != 7*time.Second {
			t.Fatalf("expected RetryAfter 7s, got %s", e.RetryAfter)
		}
	})
	t.Run("500 ServerError", func(t *testing.T) {
		_, err := serve(500, `{"error":{"code":"internal_error","message":"boom"}}`).Accounts.List(context.Background())
		assertErrorAs[*ServerError](t, err)
	})
	t.Run("non-JSON error body keeps generic message", func(t *testing.T) {
		_, err := serve(502, `<html>Bad Gateway</html>`).Accounts.List(context.Background())
		e := assertErrorAs[*ServerError](t, err)
		if !strings.Contains(e.Message, "502") {
			t.Fatalf("expected generic message mentioning the status, got %q", e.Message)
		}
	})
}

func TestConnectionErrorAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close() // nothing is listening anymore

	client, err := NewClient(WithAPIKey("omsk_test_key"), WithBaseURL(url), WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Health(context.Background())
	assertErrorAs[*ConnectionError](t, err)
}

func TestEnvKeyFallback(t *testing.T) {
	t.Setenv("OMNISOCIALS_API_KEY", "omsk_test_from_env")

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"status":"ok","version":"1.0.0","timestamp":"2026-07-14T00:00:00Z"}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(WithBaseURL(server.URL)) // no WithAPIKey: env fallback
	if err != nil {
		t.Fatalf("expected env fallback to satisfy NewClient, got: %v", err)
	}
	if _, err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if gotAuth != "Bearer omsk_test_from_env" {
		t.Fatalf("expected env API key in Authorization header, got %q", gotAuth)
	}
}

func TestExplicitKeyWinsOverEnv(t *testing.T) {
	t.Setenv("OMNISOCIALS_API_KEY", "omsk_test_from_env")

	var gotAuth string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"status":"ok","version":"1.0.0","timestamp":"2026-07-14T00:00:00Z"}`)
	}))
	if _, err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if gotAuth != "Bearer omsk_test_key" {
		t.Fatalf("expected explicit API key to win over env, got %q", gotAuth)
	}
}

func TestMissingKeyFailsConstruction(t *testing.T) {
	t.Setenv("OMNISOCIALS_API_KEY", "")
	_, err := NewClient()
	if err == nil {
		t.Fatal("expected NewClient without a key to fail")
	}
	authErr := assertErrorAs[*AuthenticationError](t, err)
	if authErr.Code != "missing_api_key" {
		t.Fatalf("expected code missing_api_key, got %q", authErr.Code)
	}
}

func TestRequestHeadersAndQuery(t *testing.T) {
	var gotUA, gotAccept, gotQuery string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotQuery = r.URL.RawQuery
		fmt.Fprint(w, `{"data":[],"pagination":{"total":0,"limit":50,"offset":10,"has_more":false}}`)
	}))

	res, err := client.Posts.List(context.Background(), &PostListParams{Status: "scheduled", Limit: 50, Offset: 10})
	if err != nil {
		t.Fatalf("Posts.List: %v", err)
	}
	// Structural: compare against the Version constant so this test never
	// needs stamping on release (set-version.mjs updates client.go).
	if gotUA != "omnisocials-go/"+Version {
		t.Fatalf("expected User-Agent omnisocials-go/%s, got %q", Version, gotUA)
	}
	if gotAccept != "application/json" {
		t.Fatalf("expected Accept application/json, got %q", gotAccept)
	}
	if !strings.Contains(gotQuery, "status=scheduled") || !strings.Contains(gotQuery, "limit=50") || !strings.Contains(gotQuery, "offset=10") {
		t.Fatalf("unexpected query string %q", gotQuery)
	}
	if res.Pagination == nil || res.Pagination.Limit != 50 {
		t.Fatalf("unexpected pagination: %+v", res.Pagination)
	}
}

func TestDeleteReturns204(t *testing.T) {
	var gotMethod, gotPath string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := client.Posts.Delete(context.Background(), "abc"); err != nil {
		t.Fatalf("expected 204 delete to succeed, got: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/posts/abc" {
		t.Fatalf("unexpected request %s %s", gotMethod, gotPath)
	}
}

func TestMediaUploadMultipart(t *testing.T) {
	fileContent := "fake image bytes"
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != fileContent {
			t.Errorf("unexpected file content %q", data)
		}
		if header.Filename != "product.jpg" {
			t.Errorf("unexpected filename %q", header.Filename)
		}
		if r.FormValue("name") != "product-hero" || r.FormValue("folder") != "Campaigns" {
			t.Errorf("unexpected form fields name=%q folder=%q", r.FormValue("name"), r.FormValue("folder"))
		}
		fmt.Fprint(w, `{"data":{"id":"9","url":"https://cdn.example.com/9.jpg","type":"image","filename":"product.jpg","size":"1.00 KB","created_at":"2026-07-14T00:00:00Z"},"compatibility":{}}`)
	}))

	res, err := client.Media.Upload(context.Background(), &MediaUploadParams{
		File:     strings.NewReader(fileContent),
		Filename: "product.jpg",
		Name:     "product-hero",
		Folder:   "Campaigns",
	})
	if err != nil {
		t.Fatalf("Media.Upload: %v", err)
	}
	if res.Data.ID != "9" || res.Data.Type != "image" {
		t.Fatalf("unexpected upload response: %+v", res.Data)
	}
}

func TestMediaUploadRequiresFile(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent when File is missing")
	}))
	if _, err := client.Media.Upload(context.Background(), &MediaUploadParams{}); err == nil {
		t.Fatal("expected an error when File is nil")
	}
}

func TestJSONBodyAndNullSerialization(t *testing.T) {
	var gotBody string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected JSON content type, got %q", ct)
		}
		fmt.Fprint(w, `{"data":{"id":"5","url":"u","type":"image","filename":"f","size":"1 KB","created_at":"c"}}`)
	}))

	_, err := client.Media.Update(context.Background(), "5", &MediaUpdateParams{
		Name:     String("renamed"),
		FolderID: Null,
	})
	if err != nil {
		t.Fatalf("Media.Update: %v", err)
	}
	if !strings.Contains(gotBody, `"name":"renamed"`) || !strings.Contains(gotBody, `"folder_id":null`) {
		t.Fatalf("expected name + explicit folder_id null in body, got %s", gotBody)
	}
}

func TestBatchAnalyticsQueryJoinsIDs(t *testing.T) {
	var gotIDs string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIDs = r.URL.Query().Get("ids")
		fmt.Fprint(w, `{"data":[{"post_id":"a","platforms":{}}],"count":1}`)
	}))

	res, err := client.Analytics.Posts(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Analytics.Posts: %v", err)
	}
	if gotIDs != "a,b,c" {
		t.Fatalf("expected ids=a,b,c, got %q", gotIDs)
	}
	if res.Count != 1 || len(res.Data) != 1 {
		t.Fatalf("unexpected batch response: %+v", res)
	}
}

// TestAllServiceMethodsExist is a compile-time inventory of the full method
// surface: 36 endpoint methods + GET /health + the webhook verify helper.
func TestAllServiceMethodsExist(t *testing.T) {
	client, err := NewClient(WithAPIKey("omsk_test_key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	methods := []any{
		// Health
		client.Health,
		// Posts (8)
		client.Posts.List, client.Posts.Get, client.Posts.RecentPlatform,
		client.Posts.Create, client.Posts.CreateAndPublish, client.Posts.Update,
		client.Posts.Delete, client.Posts.Publish,
		// Media (9)
		client.Media.List, client.Media.Get, client.Media.Upload,
		client.Media.UploadFromURL, client.Media.UploadFromBase64,
		client.Media.CreateUploadURL, client.Media.Check, client.Media.Update,
		client.Media.Delete,
		// Folders (4)
		client.Folders.List, client.Folders.Create, client.Folders.Update,
		client.Folders.Delete,
		// Accounts (2)
		client.Accounts.List, client.Accounts.Get,
		// Analytics (5)
		client.Analytics.Post, client.Analytics.Posts, client.Analytics.Overview,
		client.Analytics.Accounts, client.Analytics.BestTimes,
		// Locations (2)
		client.Locations.Search, client.Locations.Validate,
		// Webhooks (6)
		client.Webhooks.List, client.Webhooks.Get, client.Webhooks.Create,
		client.Webhooks.Update, client.Webhooks.Delete, client.Webhooks.RotateSecret,
		// Webhook signature verification
		VerifyWebhookSignature,
	}
	for i, m := range methods {
		if m == nil {
			t.Fatalf("method %d in the inventory is nil", i)
		}
	}
}
