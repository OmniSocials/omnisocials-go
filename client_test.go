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

func TestMediaEntryAltTextSerialization(t *testing.T) {
	var gotBody string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		fmt.Fprint(w, `{"data":{"id":"1","status":"draft","content":"c"}}`)
	}))

	// Plain strings and MediaURLEntry values mix in one []any list.
	_, err := client.Posts.Create(context.Background(), &PostCreateParams{
		Content:  "Sunrise over the harbor",
		Channels: []string{"mastodon"},
		MediaURLs: []any{
			"https://example.com/plain.jpg",
			MediaURLEntry{URL: "https://example.com/harbor.jpg", Alt: "A sailboat at sunrise"},
		},
	})
	if err != nil {
		t.Fatalf("Posts.Create: %v", err)
	}
	if !strings.Contains(gotBody, `"https://example.com/plain.jpg"`) ||
		!strings.Contains(gotBody, `{"url":"https://example.com/harbor.jpg","alt":"A sailboat at sunrise"}`) {
		t.Fatalf("expected mixed plain + alt media_urls entries in body, got %s", gotBody)
	}

	// MediaIDEntry omits an empty Alt.
	_, err = client.Posts.Create(context.Background(), &PostCreateParams{
		Content:  "c",
		MediaIDs: []MediaIDEntry{{ID: "42"}},
	})
	if err != nil {
		t.Fatalf("Posts.Create: %v", err)
	}
	if !strings.Contains(gotBody, `"media_ids":[{"id":"42"}]`) {
		t.Fatalf("expected media_ids entry without alt in body, got %s", gotBody)
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

func TestPostsGetApproval(t *testing.T) {
	var gotMethod, gotPath string
	body := `{"data":{"post_id":"123456","status":"rejected","workflow":{"id":"42","name":"Content approval"},"requested_by":{"id":"7d1f3c52","name":"Alex"},"requested_at":"2026-10-01T09:00:00.000Z","current_step":null,"steps":[{"order":1,"name":"Team review","require_mode":"any","status":"approved","approvers":[{"id":"2b8e6f90","name":"Sam","email":"sam@example.com","status":"approved","decided_at":"2026-10-01T10:15:00.000Z","comment":null}]},{"order":2,"name":"Client approval","require_mode":"all","status":"rejected","approvers":[{"id":"c4a09e1d","name":"Jordan","email":null,"status":"rejected","decided_at":"2026-10-01T14:30:00.000Z","comment":"The image does not match the caption"}]}],"rejection":{"by":{"id":"c4a09e1d","name":"Jordan"},"reason":"The image does not match the caption","at":"2026-10-01T14:30:00.000Z","step":2},"comments":[{"id":"5f3a2b1c","author":{"id":"c4a09e1d","name":"Jordan"},"message":"Please use the new logo","account":"instagram","created_at":"2026-10-01T14:28:00.000Z"},{"id":"1a2b3c4d","author":null,"message":"Caption edited","account":null,"created_at":"2026-09-28T22:27:30.000Z"}]}}`
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if strings.HasSuffix(r.URL.Path, "/posts/1/approval") {
			fmt.Fprint(w, `{"data":{"post_id":"1","status":"none","workflow":null,"requested_by":null,"requested_at":null,"current_step":null,"steps":[],"rejection":null,"comments":[]}}`)
			return
		}
		fmt.Fprint(w, body)
	}))

	res, err := client.Posts.GetApproval(context.Background(), "123456")
	if err != nil {
		t.Fatalf("Posts.GetApproval: %v", err)
	}
	if gotMethod != http.MethodGet || !strings.HasSuffix(gotPath, "/posts/123456/approval") {
		t.Fatalf("expected GET /posts/123456/approval, got %s %s", gotMethod, gotPath)
	}
	review := res.Data
	if review.PostID != "123456" || review.Status != "rejected" {
		t.Fatalf("unexpected review: %+v", review)
	}
	if review.Workflow == nil || review.Workflow.ID == nil || *review.Workflow.ID != "42" {
		t.Fatalf("unexpected workflow: %+v", review.Workflow)
	}
	if review.CurrentStep != nil {
		t.Fatalf("expected nil CurrentStep, got %d", *review.CurrentStep)
	}
	if len(review.Steps) != 2 || review.Steps[1].RequireMode != "all" || review.Steps[1].Status != "rejected" {
		t.Fatalf("unexpected steps: %+v", review.Steps)
	}
	first := review.Steps[0].Approvers[0]
	if first.Comment != nil || first.DecidedAt == nil || first.Email == nil || *first.Email != "sam@example.com" {
		t.Fatalf("unexpected first approver: %+v", first)
	}
	if review.Steps[1].Approvers[0].Email != nil {
		t.Fatalf("expected nil email on the second approver")
	}
	rej := review.Rejection
	if rej == nil || rej.By.ID != "c4a09e1d" || rej.Reason == nil || *rej.Reason != "The image does not match the caption" || rej.Step == nil || *rej.Step != 2 {
		t.Fatalf("unexpected rejection: %+v", rej)
	}
	if len(review.Comments) != 2 || review.Comments[0].Account == nil || *review.Comments[0].Account != "instagram" {
		t.Fatalf("unexpected comments: %+v", review.Comments)
	}
	if review.Comments[1].Author != nil || review.Comments[1].Account != nil {
		t.Fatalf("expected nil author and account on the second comment: %+v", review.Comments[1])
	}

	none, err := client.Posts.GetApproval(context.Background(), "1")
	if err != nil {
		t.Fatalf("Posts.GetApproval (none): %v", err)
	}
	if none.Data.Status != "none" || none.Data.Workflow != nil || none.Data.RequestedBy != nil ||
		none.Data.RequestedAt != nil || none.Data.Rejection != nil ||
		len(none.Data.Steps) != 0 || len(none.Data.Comments) != 0 {
		t.Fatalf("unexpected review for a post without a workflow: %+v", none.Data)
	}
}

func TestPinterestListProducts(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	body := `{"products":[{"pin_id":"813744226420795884","title":"Blue ribbed top","description":null,
		"link":"https://shop.example.com/products/blue-ribbed-top","image_url":null,
		"price":24.99,"currency":"EUR","availability":"IN_STOCK","item_id":"TOP-BLUE-M"}],
		"bookmark":"next-page","source":"catalog","catalog_access":true,
		"product_groups":[{"id":"443727193917","name":"All Products"}],"product_group_id":"443727193917"}`
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		fmt.Fprint(w, body)
	}))

	res, err := client.Pinterest.ListProducts(context.Background(), &PinterestProductListParams{
		Source: "catalog", ProductGroupID: "443727193917", Bookmark: "abc", PageSize: 50,
	})
	if err != nil {
		t.Fatalf("Pinterest.ListProducts: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/pinterest/products" {
		t.Fatalf("expected GET /pinterest/products, got %s %s", gotMethod, gotPath)
	}
	if gotQuery != "bookmark=abc&page_size=50&product_group_id=443727193917&source=catalog" {
		t.Fatalf("unexpected query string %q", gotQuery)
	}
	if res.Error != nil || len(res.Products) != 1 {
		t.Fatalf("unexpected response: %+v", res)
	}
	product := res.Products[0]
	if product.PinID != "813744226420795884" || product.Title == nil || *product.Title != "Blue ribbed top" ||
		product.Description != nil || product.Price == nil || *product.Price != 24.99 ||
		product.Currency == nil || *product.Currency != "EUR" || product.ItemID == nil || *product.ItemID != "TOP-BLUE-M" {
		t.Fatalf("unexpected product: %+v", product)
	}
	if res.Bookmark == nil || *res.Bookmark != "next-page" || res.Source != "catalog" || !res.CatalogAccess ||
		len(res.ProductGroups) != 1 || res.ProductGroups[0].ID != "443727193917" ||
		res.ProductGroupID == nil || *res.ProductGroupID != "443727193917" {
		t.Fatalf("unexpected list fields: %+v", res)
	}

	// A list that could not be read is HTTP 200 with an error object and no
	// products. Nil params send no query.
	body = `{"error":{"code":"pinterest_catalog_access_required","message":"Connect the catalog."},"catalog_access":false}`
	res, err = client.Pinterest.ListProducts(context.Background(), nil)
	if err != nil {
		t.Fatalf("Pinterest.ListProducts (error envelope): %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("expected no query for nil params, got %q", gotQuery)
	}
	if res.Error == nil || res.Error.Code != "pinterest_catalog_access_required" || res.Products != nil || res.CatalogAccess {
		t.Fatalf("unexpected error envelope: %+v", res)
	}
}

func TestPinterestValidateProduct(t *testing.T) {
	var gotPath, gotID string
	body := `{"valid":true,"pin_id":"813744226420795884","title":"Blue ribbed top","link":null,"image_url":null}`
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotID = r.URL.Path, r.URL.Query().Get("id")
		fmt.Fprint(w, body)
	}))

	link := "https://www.pinterest.com/pin/813744226420795884/"
	res, err := client.Pinterest.ValidateProduct(context.Background(), link)
	if err != nil {
		t.Fatalf("Pinterest.ValidateProduct: %v", err)
	}
	if gotPath != "/pinterest/products/validate" || gotID != link {
		t.Fatalf("unexpected request: path %q, id %q", gotPath, gotID)
	}
	if !res.Valid || res.PinID == nil || *res.PinID != "813744226420795884" || res.Title == nil || res.Link != nil || res.Unverified {
		t.Fatalf("unexpected response: %+v", res)
	}

	body = `{"valid":false,"pin_id":"813744226420795884","unverified":true,"reason":"Pinterest did not answer."}`
	res, err = client.Pinterest.ValidateProduct(context.Background(), "813744226420795884")
	if err != nil {
		t.Fatalf("Pinterest.ValidateProduct (unverified): %v", err)
	}
	if res.Valid || !res.Unverified || res.Reason == nil || *res.Reason != "Pinterest did not answer." {
		t.Fatalf("unexpected unverified response: %+v", res)
	}
}

// TestAllServiceMethodsExist is a compile-time inventory of the full method
// surface: 46 endpoint methods + GET /health + the webhook verify helper.
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
		// Posts: approval (3)
		client.Posts.Approve, client.Posts.Reject, client.Posts.GetApproval,
		// Media (9)
		client.Media.List, client.Media.Get, client.Media.Upload,
		client.Media.UploadFromURL, client.Media.UploadFromBase64,
		client.Media.CreateUploadURL, client.Media.Check, client.Media.Update,
		client.Media.Delete,
		// Folders (4)
		client.Folders.List, client.Folders.Create, client.Folders.Update,
		client.Folders.Delete,
		// Hashtag sets (5)
		client.HashtagSets.List, client.HashtagSets.Get, client.HashtagSets.Create,
		client.HashtagSets.Update, client.HashtagSets.Delete,
		// Accounts (2)
		client.Accounts.List, client.Accounts.Get,
		// Analytics (5)
		client.Analytics.Post, client.Analytics.Posts, client.Analytics.Overview,
		client.Analytics.Accounts, client.Analytics.BestTimes,
		// Locations (2)
		client.Locations.Search, client.Locations.Validate,
		// Pinterest (2)
		client.Pinterest.ListProducts, client.Pinterest.ValidateProduct,
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
