// Package omnisocials is the official Go client for the OmniSocials API
// (https://api.omnisocials.com/v1): schedule and publish posts to Instagram,
// Facebook, LinkedIn, YouTube, TikTok, X, Pinterest, Bluesky, Threads,
// Mastodon, and Google Business from one API.
//
// Quickstart:
//
//	client, err := omnisocials.NewClient() // reads OMNISOCIALS_API_KEY from env
//	if err != nil {
//		log.Fatal(err)
//	}
//	post, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
//		Content:     "Hello from the SDK",
//		Channels:    []string{"instagram", "linkedin"},
//		ScheduledAt: "2026-08-01T09:00:00Z",
//	})
//
// Full documentation: https://docs.omnisocials.com
package omnisocials

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version.
const Version = "0.1.0"

const (
	defaultBaseURL    = "https://api.omnisocials.com/v1"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2
	userAgent         = "omnisocials-go/" + Version

	maxBackoff    = 30 * time.Second
	maxRetryAfter = 60 * time.Second
)

// Client is the OmniSocials API client. Create one with NewClient and use the
// service fields (Posts, Media, ...) to call the API. A Client is safe for
// concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client

	// Posts covers /posts: list, get, create, create-and-publish, update,
	// delete, publish, and recent-platform.
	Posts *PostsService
	// Media covers /media: list, get, uploads (multipart, URL, base64,
	// presigned), check, update, and delete.
	Media *MediaService
	// Folders covers /folders: list, create, update, delete.
	Folders *FoldersService
	// Accounts covers /accounts: the workspace's connected social accounts.
	Accounts *AccountsService
	// Analytics covers /analytics: post stats (single + batch), overview,
	// account stats, and best times to post.
	Analytics *AnalyticsService
	// Locations covers /locations: Instagram place tagging search + validate.
	Locations *LocationsService
	// Audio covers /audio: Instagram Reels licensed audio search.
	Audio *AudioService
	// Webhooks covers /webhooks: endpoint management + secret rotation.
	Webhooks *WebhooksService
	// Inbox covers /inbox: social inbox conversations, messages, mark-read,
	// and reply.
	Inbox *InboxService
}

// Option configures a Client. Pass options to NewClient.
type Option func(*Client)

// WithAPIKey sets the API key (`omsk_live_*` / `omsk_test_*`). It takes
// precedence over the OMNISOCIALS_API_KEY environment variable.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) { c.apiKey = apiKey }
}

// WithBaseURL overrides the API base URL
// (default https://api.omnisocials.com/v1).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = baseURL }
}

// WithTimeout sets the per-request timeout (default 30s). Each retry attempt
// gets the full timeout. A non-positive value disables the SDK timeout (the
// caller's context still applies).
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) { c.timeout = timeout }
}

// WithMaxRetries sets how many times a failed request is retried on 429, 5xx,
// and connection errors (default 2, i.e. up to 3 attempts total).
func WithMaxRetries(maxRetries int) Option {
	return func(c *Client) {
		if maxRetries < 0 {
			maxRetries = 0
		}
		c.maxRetries = maxRetries
	}
}

// WithHTTPClient supplies a custom *http.Client (proxies, custom transports,
// instrumentation). The SDK still applies its own per-attempt timeout via
// context unless disabled with WithTimeout.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// NewClient creates an OmniSocials API client.
//
// The API key is taken from WithAPIKey when given, otherwise from the
// OMNISOCIALS_API_KEY environment variable. When neither is set, NewClient
// returns an *AuthenticationError. Create a key in the OmniSocials app under
// Settings -> API Keys.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    defaultBaseURL,
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv("OMNISOCIALS_API_KEY")
	}
	if c.apiKey == "" {
		return nil, &AuthenticationError{APIError: APIError{
			Status: http.StatusUnauthorized,
			Code:   "missing_api_key",
			Message: "No API key provided. Pass one with omnisocials.WithAPIKey(\"omsk_live_...\") " +
				"or set the OMNISOCIALS_API_KEY environment variable. Create a key in the " +
				"OmniSocials app under Settings -> API Keys.",
		}}
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}

	c.Posts = &PostsService{client: c}
	c.Media = &MediaService{client: c}
	c.Folders = &FoldersService{client: c}
	c.Accounts = &AccountsService{client: c}
	c.Analytics = &AnalyticsService{client: c}
	c.Locations = &LocationsService{client: c}
	c.Audio = &AudioService{client: c}
	c.Webhooks = &WebhooksService{client: c}
	c.Inbox = &InboxService{client: c}
	return c, nil
}

// Health calls `GET /health` (no scopes required).
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var out HealthResponse
	if err := c.get(ctx, "/health", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- HTTP verb helpers used by the services -------------------------------

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, "", out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}

func (c *Client) patch(ctx context.Context, path string, body, out any) error {
	return c.doJSON(ctx, http.MethodPatch, path, body, out)
}

func (c *Client) del(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, "", nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	var contentType string
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("omnisocials: failed to encode request body: %w", err)
		}
		contentType = "application/json"
	}
	return c.do(ctx, method, path, nil, data, contentType, out)
}

// apiResponse is a fully-read HTTP response.
type apiResponse struct {
	status int
	header http.Header
	body   []byte
}

// do runs the core request loop: build the URL, send the request with a
// per-attempt timeout, and retry 429 / 5xx / connection errors with
// exponential backoff + jitter (honoring Retry-After when present). The
// response body is decoded into out as-is (the API's `{ data, ... }` envelope
// is preserved); 204 / empty bodies leave out untouched.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, contentType string, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	if _, err := url.Parse(endpoint); err != nil {
		// A malformed base URL is not retryable.
		return &ConnectionError{Message: fmt.Sprintf("invalid request URL %q: %v", endpoint, err), Err: err}
	}

	for attempt := 0; ; attempt++ {
		resp, err := c.send(ctx, method, endpoint, body, contentType)
		if err != nil {
			// Network failure or per-attempt timeout: retryable, unless the
			// caller's own context is done.
			if ctx.Err() == nil && attempt < c.maxRetries {
				if sleepErr := sleepContext(ctx, c.backoff(attempt, 0, false)); sleepErr == nil {
					continue
				}
			}
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return &ConnectionError{
					Message: fmt.Sprintf("request timed out after %s (%s %s)", c.timeout, method, endpoint),
					Err:     err,
				}
			}
			return &ConnectionError{
				Message: fmt.Sprintf("connection error (%s %s): %v", method, endpoint, err),
				Err:     err,
			}
		}

		if resp.status >= 200 && resp.status < 300 {
			if out == nil || resp.status == http.StatusNoContent || len(resp.body) == 0 {
				return nil
			}
			if err := json.Unmarshal(resp.body, out); err != nil {
				return fmt.Errorf("omnisocials: failed to decode %s %s response: %w", method, endpoint, err)
			}
			return nil
		}

		retryAfter, hasRetryAfter := parseRetryAfter(resp.header.Get("Retry-After"))
		if (resp.status == http.StatusTooManyRequests || resp.status >= 500) && attempt < c.maxRetries {
			if sleepErr := sleepContext(ctx, c.backoff(attempt, retryAfter, hasRetryAfter)); sleepErr != nil {
				return &ConnectionError{Message: "request canceled while waiting to retry: " + sleepErr.Error(), Err: sleepErr}
			}
			continue
		}

		return responseToError(resp, retryAfter)
	}
}

// send performs one HTTP attempt and reads the full response body while the
// per-attempt timeout is still in effect.
func (c *Client) send(ctx context.Context, method, endpoint string, body []byte, contentType string) (*apiResponse, error) {
	attemptCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		attemptCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return &apiResponse{status: res.StatusCode, header: res.Header, body: data}, nil
}

// backoff returns the wait before retry number attempt+1. Exponential
// (0.5s, 1s, 2s, ...) with 75%-125% jitter, capped at 30s; a parsed
// Retry-After header wins (capped at 60s).
func (c *Client) backoff(attempt int, retryAfter time.Duration, hasRetryAfter bool) time.Duration {
	if hasRetryAfter {
		if retryAfter > maxRetryAfter {
			return maxRetryAfter
		}
		return retryAfter
	}
	if attempt > 10 {
		attempt = 10 // avoid shift overflow with large custom retry counts
	}
	base := time.Duration(1<<uint(attempt)) * 500 * time.Millisecond
	jitter := 0.75 + rand.Float64()*0.5
	d := time.Duration(float64(base) * jitter)
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

// responseToError builds the typed error for a non-2xx response.
func responseToError(resp *apiResponse, retryAfter time.Duration) error {
	code := ""
	message := fmt.Sprintf("Request failed with status %d.", resp.status)
	var parsedBody any
	if len(resp.body) > 0 {
		var decoded any
		if err := json.Unmarshal(resp.body, &decoded); err == nil {
			parsedBody = decoded
			if obj, ok := decoded.(map[string]any); ok {
				switch errField := obj["error"].(type) {
				case map[string]any:
					if s, ok := errField["code"].(string); ok {
						code = s
					}
					if s, ok := errField["message"].(string); ok {
						message = s
					}
				case string:
					message = errField
				}
			}
		}
		// Non-JSON error bodies (e.g. an HTML 413 page from the CDN) keep the
		// generic message and a nil Body, matching the other SDKs.
	}
	return newAPIError(resp.status, code, message, parsedBody, retryAfter)
}

// parseRetryAfter parses a Retry-After header value (delta-seconds or
// HTTP-date). The second return value reports whether a usable value was
// present.
func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds * float64(time.Second)), true
	}
	if when, err := http.ParseTime(value); err == nil {
		d := time.Until(when)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// jsonBody substitutes an empty JSON object for a nil params pointer so the
// API returns its own validation error instead of the SDK panicking or
// sending `null`.
func jsonBody[T any](params *T) any {
	if params == nil {
		return map[string]any{}
	}
	return params
}
