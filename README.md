# OmniSocials Go SDK

The official Go client for the [OmniSocials API](https://docs.omnisocials.com). Schedule and publish posts to Instagram, Facebook, LinkedIn, YouTube, TikTok, X, Pinterest, Bluesky, Threads, Mastodon, and Google Business from one API.

- Standard library only, no dependencies (net/http, crypto/hmac)
- Context-first methods and typed request/response structs
- Automatic retries with exponential backoff, configurable timeouts
- Rich error types matched with `errors.As`, plus a webhook signature verification helper
- Go 1.21+

## Installation

```bash
go get github.com/OmniSocials/omnisocials-go
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	omnisocials "github.com/OmniSocials/omnisocials-go"
)

func main() {
	client, err := omnisocials.NewClient() // reads OMNISOCIALS_API_KEY from env
	if err != nil {
		log.Fatal(err)
	}

	post, err := client.Posts.Create(context.Background(), &omnisocials.PostCreateParams{
		Content:     "Hello from the SDK",
		Channels:    []string{"instagram", "linkedin"},
		ScheduledAt: "2026-08-01T09:00:00Z",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(post.Data.ID, post.Data.Status)
}
```

## Authentication

Create an API key in the OmniSocials app under **Settings -> API Keys**. Keys look like `omsk_live_...` (or `omsk_test_...`).

The client reads `OMNISOCIALS_API_KEY` from the environment, or you can pass it explicitly:

```go
client, err := omnisocials.NewClient(omnisocials.WithAPIKey("omsk_live_..."))
```

Constructing a client without a key returns an `*omnisocials.AuthenticationError` right away.

## Configuration

```go
client, err := omnisocials.NewClient(
	omnisocials.WithAPIKey("omsk_live_..."),
	omnisocials.WithBaseURL("https://api.omnisocials.com/v1"), // default
	omnisocials.WithTimeout(30*time.Second),                   // per-request timeout (default 30s)
	omnisocials.WithMaxRetries(2),                             // retries on 429 / 5xx / network errors (default 2)
	omnisocials.WithHTTPClient(customHTTPClient),              // optional custom *http.Client
)
```

Retries use exponential backoff (0.5s, 1s, 2s, ...) with jitter and honor the `Retry-After` header. Other 4xx responses are never retried. Every method takes a `context.Context`, so you can add deadlines and cancellation on top.

## Rate limits

The API allows **100 requests per minute** per API key. When you exceed it, the SDK retries automatically (respecting `Retry-After`); if retries are exhausted it returns a `*RateLimitError` whose `RetryAfter` field holds the wait as a `time.Duration`.

## Return values

Methods return the parsed response body as-is: single items come back as `ItemResponse[T]` (`{ "data": {...} }`), lists as `ListResponse[T]` (`{ "data": [...], "pagination": {...} }`), and some responses carry extra sibling fields (media uploads include `Compatibility`, PDF uploads include `Slides` and `MediaIDs`, post creates targeting X with a URL in the text include `Warnings`). Endpoints that respond `204 No Content` (deletes) return only an `error`.

## Posts

### Schedule a post

```go
res, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "New drop this Friday",
	Channels:    []string{"instagram", "facebook", "linkedin"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	MediaURLs:   []string{"https://example.com/teaser.jpg"},
})
fmt.Println(res.Data.ID, res.Data.Status)
```

Omit `ScheduledAt` to create a draft. Use a map for per-platform captions:

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content: map[string]string{
		"default": "New drop this Friday",
		"x":       "New drop this Friday. RT to spread the word",
	},
	Channels:    []string{"instagram", "x"},
	ScheduledAt: "2026-08-01T09:00:00Z",
})
```

### Publish immediately

```go
_, err = client.Posts.CreateAndPublish(ctx, &omnisocials.PostCreateParams{
	Content:  "Going live right now",
	Channels: []string{"x", "bluesky"},
})
```

### Per-media alt text

Every `MediaURLs` / `MediaIDs` entry accepts either a plain string or a `MediaURLEntry` / `MediaIDEntry` with an `Alt` accessibility description (max 1500 chars). Alt text is delivered to Mastodon (media description), Bluesky (embed alt), X (photos and GIFs), Pinterest (pin alt text), Instagram (images), and LinkedIn (images). Strings and entry structs can be mixed in a `[]any`, and the same shapes work in per-platform maps and `ThreadPart` media.

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Sunrise over the harbor",
	Channels:    []string{"mastodon", "bluesky"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	MediaURLs: []omnisocials.MediaURLEntry{
		{
			URL: "https://example.com/harbor.jpg",
			Alt: "A small sailboat crossing a calm harbor at sunrise, sky in deep orange",
		},
	},
})
```

### Post with platform-specific options

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Behind the scenes of our summer shoot",
	Channels:    []string{"instagram", "youtube", "x"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	MediaURLs:   []string{"https://example.com/bts.mp4"},
	Instagram:   map[string]any{"share_to_feed": true},
	YouTube:     map[string]any{"title": "Summer shoot BTS", "privacy": "public"},
	X: &omnisocials.XPostOptions{
		ReplySettings: "following",
		MadeWithAI:    omnisocials.Bool(false),
	},
})
```

### Chained threads (X, Bluesky, Mastodon, Threads)

Provide 2 to 25 `ThreadParts` to publish a chained thread instead of a single tweet. Each part is capped at 280 characters and can carry its own media. The same `ThreadPart` shape works for `Bluesky` (300 chars per part), `Mastodon` (500 chars per part) and `Threads` (Meta Threads: 2 to 25 parts, 500 characters per part, up to 10 media per part; parts after the first publish as replies to the previous part, and the Threads caption is taken from part 1).

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "How we grew to 10k followers in 90 days",
	Channels:    []string{"x"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	X: &omnisocials.XPostOptions{
		ThreadParts: []omnisocials.ThreadPart{
			{Text: "How we grew to 10k followers in 90 days. A thread:"},
			{Text: "1. We posted every single day, even when it felt pointless."},
			{Text: "2. We replied to every comment within an hour."},
			{Text: "3. Full breakdown on our blog. Link in bio."},
		},
	},
})
```

```go
// Meta Threads chain with a carousel on the first part
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Behind the scenes of our summer shoot",
	Channels: []string{"threads"},
	Threads: &omnisocials.ThreadsPostOptions{
		ThreadParts: []omnisocials.ThreadPart{
			{Text: "Behind the scenes of our summer shoot. A few highlights:", MediaURLs: []string{"https://example.com/shoot-1.jpg", "https://example.com/shoot-2.jpg"}},
			{Text: "Day one: scouting locations at sunrise."},
			{Text: "Day two: the full crew, 14 hours, zero regrets."},
		},
	},
})
```

On update, set `ThreadParts: omnisocials.Null` to clear thread mode (revert to a single post); leave it nil to keep the existing thread untouched. The same works for `Bluesky`, `Mastodon` and `Threads`:

```go
_, err = client.Posts.Update(ctx, postID, &omnisocials.PostUpdateParams{
	X:       &omnisocials.XPostOptionsUpdate{ThreadParts: omnisocials.Null},
	Threads: &omnisocials.ThreadsPostOptionsUpdate{ThreadParts: omnisocials.Null},
})
```

### X link posts use credits

X bills API posts whose text contains a URL at a premium, and OmniSocials passes that fee through as prepaid credits (20 credits per URL-containing tweet; threads billed per part with a link). When a create targets X and the text contains a URL, the response's `Warnings` field carries a `PostWarning` with code `x_url_post_credits`:

```go
res, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Read the full story: https://example.com/post",
	Channels: []string{"x"},
})
for _, w := range res.Warnings {
	if w.Code == "x_url_post_credits" {
		fmt.Println(w.CreditsRequired, w.Message)
	}
}
```

From `EnforceFrom` (2026-08-14) the balance is checked at publish time, but credits are only deducted after the post successfully publishes (a failed publish is never charged). If the balance can't cover it, only the X target fails (other platforms publish normally); top up in the dashboard under Settings -> Organisation -> Billing -> Credits, then `Posts.Retry`. Posts without links, analytics, and media on X stay free. There is no API endpoint for credits — they are managed in the dashboard.

**Schedule-time gate.** Separately, every scheduled X link post *reserves* its cost until it publishes (shown as an orange "reserved" slice on the dashboard's credits page). `Posts.Create` (non-draft), `Posts.CreateAndPublish`, `Posts.Update` (when it schedules or edits a scheduled X link post), and `Posts.Publish` all refuse up front with a 402 error whose `Code` is `x_credits_insufficient` when this post's cost plus what's already reserved by other scheduled X link posts would exceed the balance. 402 isn't one of the SDK's typed error subclasses, so match it against the base `*APIError` and check `Code`:

```go
_, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Read the full story: https://example.com/post",
	Channels:    []string{"x"},
	ScheduledAt: "2026-08-15T09:00:00Z",
})
var apiErr *omnisocials.APIError
if errors.As(err, &apiErr) && apiErr.Code == "x_credits_insufficient" {
	log.Printf("not enough credits: %v", apiErr.Body)
}
```

Drafts are never gated (the check runs when a draft is scheduled or published), and posts publishing before `EnforceFrom` are never gated. This gate is separate from the `x_url_post_credits` warning above (create-time, non-blocking) and from the publish-time-only-X-target-fails behavior.

### List, get, update, publish, retry, delete

```go
list, err := client.Posts.List(ctx, &omnisocials.PostListParams{Status: "scheduled", Limit: 50})
one, err := client.Posts.Get(ctx, list.Data[0].ID)
_, err = client.Posts.Update(ctx, one.Data.ID, &omnisocials.PostUpdateParams{ScheduledAt: "2026-08-02T10:00:00Z"})
_, err = client.Posts.Publish(ctx, one.Data.ID) // publish a draft/scheduled post now
_, err = client.Posts.Retry(ctx, one.Data.ID)   // retry only the failed platforms of a failed/warning post
err = client.Posts.Delete(ctx, one.Data.ID)     // 204 on success
```

`Retry` re-publishes only the platforms that failed, on the same post; platforms that already succeeded are never posted again. It is asynchronous: a 200 means the retry is queued, so poll `Get` for the outcome. Max 3 retries per platform.

### Recent platform posts

Fetch recent posts live from the connected platform APIs, including content published outside OmniSocials. Useful for brand-new workspaces where `List` is empty. Requires the `analytics:read` scope. Each record includes `duration_seconds` (integer, nullable): the video length in whole seconds where the platform reports it — currently TikTok and YouTube; `null` for images and for platforms that don't expose it.

```go
recent, err := client.Posts.RecentPlatform(ctx, &omnisocials.RecentPlatformPostsParams{
	Limit:     10,
	Platforms: []string{"instagram", "x"},
})
```

## Media

### Upload from a URL (recommended, up to 1GB)

```go
upload, err := client.Media.UploadFromURL(ctx, &omnisocials.MediaUploadFromURLParams{
	URL:    "https://example.com/launch-video.mp4",
	Name:   "launch-video-v2",
	Folder: "Campaigns",
})
fmt.Println(upload.Data.ID, upload.Compatibility)
```

Videos over 100MB are processed in the background and come back with status `"processing"`. Every upload response includes a `Compatibility` block listing connected platforms that would reject the file.

### Upload a local file (multipart)

`Media.Upload` takes any `io.Reader` plus a filename and builds the multipart request for you:

```go
file, err := os.Open("./photos/product.jpg")
if err != nil {
	log.Fatal(err)
}
defer file.Close()

res, err := client.Media.Upload(ctx, &omnisocials.MediaUploadParams{
	File:     file,
	Filename: "product.jpg",
	Name:     "product-hero",
})
```

Direct multipart uploads are capped at 100MB by the CDN; use `UploadFromURL` or the presigned flow below for bigger files.

### Upload from base64

```go
_, err = client.Media.UploadFromBase64(ctx, &omnisocials.MediaUploadFromBase64Params{
	Data:     base64String, // no data URI prefix
	MimeType: "image/png",
	Filename: "chart.png",
})
```

### PDF carousels

Uploading a PDF rasterizes it into one image slide per page (max 20). The response carries `Slides` and `MediaIDs` alongside `Data` (the first slide). Pass ALL of `MediaIDs`, in order, to `Posts.Create` to post the deck as a carousel (a native swipeable document on LinkedIn, an image carousel elsewhere).

```go
pdf, err := client.Media.UploadFromURL(ctx, &omnisocials.MediaUploadFromURLParams{URL: "https://example.com/deck.pdf"})
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Our Q3 strategy deck",
	Channels:    []string{"linkedin"},
	MediaIDs:    pdf.MediaIDs,
	ScheduledAt: "2026-08-01T09:00:00Z",
})
```

### Presigned uploads for large files (up to 1GB)

`CreateUploadURL` mints a one-time upload URL. POST the file to it as multipart form data (field name `file`) within `ExpiresInSeconds` (600s); the second request needs no auth headers because the single-use token is in the URL. The response of that second request is the created media item (or `media_ids` for a PDF).

```go
presigned, err := client.Media.CreateUploadURL(ctx)
if err != nil {
	log.Fatal(err)
}

var buf bytes.Buffer
writer := multipart.NewWriter(&buf)
part, _ := writer.CreateFormFile("file", "big-video.mp4")
f, _ := os.Open("./big-video.mp4")
io.Copy(part, f)
f.Close()
writer.Close()

req, _ := http.NewRequest(http.MethodPost, presigned.UploadURL, &buf)
req.Header.Set("Content-Type", writer.FormDataContentType())
resp, err := http.DefaultClient.Do(req)
```

### Preflight compatibility check

Check a file against the workspace's connected platforms before uploading. Provide one of `URL`, `MediaID`, or `SizeBytes` + `Mime`.

```go
check, err := client.Media.Check(ctx, &omnisocials.MediaCheckParams{URL: "https://example.com/huge.mov"})
check, err = client.Media.Check(ctx, &omnisocials.MediaCheckParams{SizeBytes: 300_000_000, Mime: "video/quicktime"})
```

### List, get, rename, move, delete

```go
items, err := client.Media.List(ctx, &omnisocials.MediaListParams{Search: "hero", Limit: 20})
_, err = client.Media.Update(ctx, items.Data[0].ID, &omnisocials.MediaUpdateParams{
	Name:     omnisocials.String("hero-v2"),
	FolderID: "12", // or omnisocials.Null to move back to the root
})
_, err = client.Media.Get(ctx, items.Data[0].ID)
err = client.Media.Delete(ctx, items.Data[0].ID) // 409 media_in_use if attached to a scheduled post
```

## Folders

```go
folders, err := client.Folders.List(ctx) // flat; build the tree via ParentID
folder, err := client.Folders.Create(ctx, &omnisocials.FolderCreateParams{Name: "Campaigns"})
_, err = client.Folders.Update(ctx, folder.Data.ID, &omnisocials.FolderUpdateParams{Name: "Campaigns 2026"})
err = client.Folders.Delete(ctx, folder.Data.ID) // files move to root, subfolders move up
```

## Hashtag Sets

Save reusable hashtag groups and apply them to posts at create time. Uses the `posts:read` / `posts:write` scopes.

```go
set, err := client.HashtagSets.Create(ctx, &omnisocials.HashtagSetCreateParams{
	Name:     "Launch",
	Hashtags: []string{"saas", "buildinpublic", "startup"}, // or one string: "#saas #buildinpublic #startup"
})
fmt.Println(set.Data.Preview) // "#saas #buildinpublic #startup"

sets, err := client.HashtagSets.List(ctx)
_, err = client.HashtagSets.Get(ctx, set.Data.ID)
_, err = client.HashtagSets.Update(ctx, set.Data.ID, &omnisocials.HashtagSetUpdateParams{
	Hashtags: []string{"saas", "founder"}, // replaces the full list
})
err = client.HashtagSets.Delete(ctx, set.Data.ID)
```

Apply a set when creating a post with `HashtagSet` (the set name, case-insensitive) or `HashtagSetID`. The set is applied once at create time and tags already in the caption are skipped. `HashtagPlacement` is `"caption_append"` (default) or `"first_comment"`, and `HashtagPlatforms` restricts the hashtags to a subset of the post's channels. Instagram's 30-hashtag cap returns error code `hashtag_limit_exceeded`.

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:          "Launch day!",
	Channels:         []string{"instagram", "x"},
	ScheduledAt:      "2026-08-01T09:00:00Z",
	HashtagSet:       "Launch",
	HashtagPlacement: "first_comment",
	HashtagPlatforms: []string{"instagram"},
})
```

## Accounts

```go
accounts, err := client.Accounts.List(ctx)
for _, account := range accounts.Data {
	fmt.Println(account.Platform, account.Username, account.Status)
	if account.NeedsReconnect {
		reason := ""
		if account.ReauthReason != nil {
			reason = *account.ReauthReason
		}
		log.Printf("%s needs a reconnect: %s", account.Platform, reason)
	}
}
ig, err := client.Accounts.Get(ctx, accounts.Data[0].ID)
```

## Analytics

```go
// One post's latest per-platform metrics
stats, err := client.Analytics.Post(ctx, "post_id")
fmt.Println(stats.Data.Platforms["instagram"].Metrics)

// Batch: up to 100 posts in one call
batch, err := client.Analytics.Posts(ctx, []string{"id1", "id2", "id3"})

// Workspace-wide overview
overview, err := client.Analytics.Overview(ctx, &omnisocials.AnalyticsOverviewParams{Period: "30d"})
fmt.Println(overview.Data.TotalImpressions, overview.Data.TotalEngagement)

// Account-level stats (followers etc)
accountStats, err := client.Analytics.Accounts(ctx, &omnisocials.AccountAnalyticsParams{Platform: "instagram"})
```

### Best times to post

```go
best, err := client.Analytics.BestTimes(ctx, &omnisocials.BestTimesParams{
	Platform: "instagram",
	Timezone: "Europe/Amsterdam",
})
```

## Locations (Instagram place tagging)

```go
results, err := client.Locations.Search(ctx, "Griffith Observatory")
place := results.Data[0]

check, err := client.Locations.Validate(ctx, place.ID)
if check.Valid {
	_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
		Content:     "Golden hour at the observatory",
		Channels:    []string{"instagram"},
		MediaURLs:   []string{"https://example.com/observatory.jpg"},
		LocationID:  place.ID,
		ScheduledAt: "2026-08-01T18:30:00Z",
	})
}
```

## Webhooks

### Manage endpoints

```go
webhook, err := client.Webhooks.Create(ctx, &omnisocials.WebhookCreateParams{
	URL:    "https://example.com/omnisocials/webhook",
	Events: []string{"post.published", "post.failed"},
})
fmt.Println(webhook.Data.Secret) // save it, it is only shown once

_, err = client.Webhooks.List(ctx)
_, err = client.Webhooks.Get(ctx, webhook.Data.ID)
_, err = client.Webhooks.Update(ctx, webhook.Data.ID, &omnisocials.WebhookUpdateParams{IsActive: omnisocials.Bool(false)})
rotated, err := client.Webhooks.RotateSecret(ctx, webhook.Data.ID)
fmt.Println(rotated.Data.Secret) // the old secret stops working
err = client.Webhooks.Delete(ctx, webhook.Data.ID)
```

### Verify deliveries (net/http example)

Every delivery is signed with your webhook secret. The `X-OmniSocials-Signature` header has the form `t=<unix>,v1=<hex>` where the hex value is an HMAC-SHA256 of `"{timestamp}.{rawBody}"`. Always verify against the RAW request body:

```go
package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	omnisocials "github.com/OmniSocials/omnisocials-go"
)

func main() {
	secret := os.Getenv("OMNISOCIALS_WEBHOOK_SECRET")

	http.HandleFunc("/omnisocials/webhook", func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body) // keep the raw body!
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		event, err := omnisocials.VerifyWebhookSignature(
			payload,
			r.Header.Get("X-OmniSocials-Signature"),
			secret,
			5*time.Minute, // tolerance; pass 0 for the default (5 minutes)
		)
		if err != nil {
			var verr *omnisocials.WebhookVerificationError
			if errors.As(err, &verr) {
				http.Error(w, "invalid signature", http.StatusBadRequest)
				return
			}
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		data, _ := event["data"].(map[string]any)
		switch event["type"] {
		case "post.published":
			log.Println("Published:", data["post_id"], data["targets"])
		case "post.failed":
			log.Println("Failed:", data["post_id"])
		}
		w.WriteHeader(http.StatusOK)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

`VerifyWebhookSignature` uses a constant-time comparison, rejects timestamps older than the tolerance (replay protection), returns a `*WebhookVerificationError` on any failure, and returns the parsed event on success.

## Inbox

Social inbox conversations (DMs, comments, and mentions) across connected platforms: Instagram, Facebook, LinkedIn, TikTok (video comments only), and X (DMs). TikTok replies are comments only and capped at 150 characters.

```go
conversations, err := client.Inbox.ListConversations(ctx, &omnisocials.InboxListParams{
	Platform: "instagram",
	Unread:   omnisocials.Bool(true),
	Limit:    20,
})
for _, c := range conversations.Data {
	fmt.Println(c.Platform, c.Type, c.Participant.Username, c.UnreadCount)
}

// Cursor pagination: pass the previous page's NextCursor while HasMore is true.
if conversations.Pagination != nil && conversations.Pagination.HasMore {
	next, err := client.Inbox.ListConversations(ctx, &omnisocials.InboxListParams{
		Cursor: *conversations.Pagination.NextCursor,
	})
	_ = next
}

thread, err := client.Inbox.GetMessages(ctx, conversations.Data[0].ConversationID, nil)
_, err = client.Inbox.MarkRead(ctx, conversations.Data[0].ConversationID)

reply, err := client.Inbox.Reply(ctx, conversations.Data[0].ConversationID, &omnisocials.InboxReplyParams{
	Text: "Thanks for reaching out!",
})
fmt.Println(reply.Data.ID, reply.Data.Direction)
```

**X (Twitter) DM credits.** X DMs are supported once a workspace opts in (dashboard-only). Replying on an X conversation costs **2 prepaid credits** per send (X's per-request fee), debited before the send and auto-refunded if it fails. `Reply` returns two X-specific 402 codes, neither of which is one of the SDK's typed error subclasses — match them against the base `*APIError`:

```go
_, err = client.Inbox.Reply(ctx, xConversationID, &omnisocials.InboxReplyParams{Text: "On it!"})
var apiErr *omnisocials.APIError
if errors.As(err, &apiErr) {
	switch apiErr.Code {
	case "insufficient_credits":
		log.Println("balance can't cover the 2-credit send")
	case "x_inbox_suspended":
		log.Println("workspace's X inbox is suspended, top up and re-enable in the dashboard")
	}
}
```

## Health

```go
health, err := client.Health(ctx) // {Status: "ok", Version: "1.0.0", Timestamp: "..."}
```

## Error handling

All API failures are typed and matched with `errors.As`. Non-2xx responses produce an `*APIError` wrapper with `Status`, `Code`, `Message`, and the parsed `Body`:

| Type | Status | Typical API codes |
|---|---|---|
| `*ValidationError` | 400 / 422 | `validation_error`, `platform_not_connected`, `invalid_file_type` |
| `*AuthenticationError` | 401 | `unauthorized`, `invalid_api_key` |
| `*PermissionDeniedError` | 403 | `forbidden`, `insufficient_scope` |
| `*NotFoundError` | 404 | `not_found` |
| `*RateLimitError` | 429 | `rate_limit_exceeded` (exposes `RetryAfter`) |
| `*ServerError` | >= 500 | `internal_error` |
| `*ConnectionError` | n/a | network failure or timeout |
| `*WebhookVerificationError` | n/a | invalid webhook signature |

Every typed wrapper also unwraps to the base `*APIError`, so a single `errors.As` catch-all works too:

```go
_, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Hi",
	Channels: []string{"instagram"},
})
if err != nil {
	var rateLimited *omnisocials.RateLimitError
	var validation *omnisocials.ValidationError
	var connection *omnisocials.ConnectionError
	var apiErr *omnisocials.APIError

	switch {
	case errors.As(err, &rateLimited):
		log.Printf("rate limited, retry in %s", rateLimited.RetryAfter)
	case errors.As(err, &validation):
		log.Printf("bad request (%s): %s", validation.Code, validation.Message)
	case errors.As(err, &connection):
		log.Printf("network problem: %v", connection)
	case errors.As(err, &apiErr):
		log.Printf("API error %d (%s): %s", apiErr.Status, apiErr.Code, apiErr.Message)
	default:
		log.Fatal(err)
	}
}
```

## API scopes

Each API key carries scopes: `posts:read`, `posts:write`, `media:write`, `accounts:read`, `analytics:read`, `webhooks:manage`. A call with a missing scope returns a `*PermissionDeniedError` with code `insufficient_scope`.

## Documentation

Full API reference and guides: [https://docs.omnisocials.com](https://docs.omnisocials.com)

## License

MIT
