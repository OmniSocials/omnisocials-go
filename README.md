# OmniSocials Go SDK

The official Go SDK for OmniSocials: post, schedule, reply and report on 13 social platforms (Instagram, Facebook, LinkedIn, X, TikTok, YouTube, Threads, Pinterest, Bluesky, Mastodon, Google Business, Telegram and Discord) from one API. Guide: [omnisocials.com/sdk/go](https://omnisocials.com/sdk/go)

- Standard library only, no dependencies (net/http, crypto/hmac)
- Context-first methods and typed request/response structs
- Automatic retries with exponential backoff, configurable timeouts
- Rich error types matched with `errors.As`, plus a webhook signature verification helper
- Go 1.26+

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

Create an API key in the OmniSocials app under **Settings -> API Keys**. Keys start with `omsk_live_`. There are no test keys and no test mode: every key works on your real connected accounts. To try things safely, leave out `ScheduledAt` so the post stays a draft (a draft never publishes on its own), or use a separate workspace with test accounts.

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

### Planned drafts

Set `Draft: omnisocials.Bool(true)` with `ScheduledAt` to store the post as a draft that keeps its time. It shows on the dashboard calendar at that slot with a dashed outline and the scheduler never publishes it: a person schedules it from the composer, or you do with `Posts.Update` and `Draft: omnisocials.Bool(false)`. Use it when an agent proposes a slot that a human should confirm. A draft is not allowed with `publish_now` or `ApprovalWorkflowID` (`400 validation_error`). On `Update`, `Draft` false without `ScheduledAt` is a `400 validation_error` when the draft has no time or its time is in the past (pass a new `ScheduledAt` then), and `Draft` true on a scheduled post unschedules it while keeping the time.

```go
proposal, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Proposed for Friday morning",
	Channels:    []string{"instagram", "linkedin"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	Draft:       omnisocials.Bool(true),
})
fmt.Println(proposal.Data.Status, *proposal.Data.ScheduleAt) // draft, the time is kept

// The human agrees: schedule it at the time it already carries
_, err = client.Posts.Update(ctx, proposal.Data.ID, &omnisocials.PostUpdateParams{Draft: omnisocials.Bool(false)})
```

### Comment-to-DM automations

Answer a keyword in an Instagram comment with a DM, once per person, with an optional public reply under the comment. Set `CommentDMAutomation` on `Create` (Instagram must be in `Channels`) and the automation is bound to that post; it starts listening once the post is live. Needs the opt-in `automations:write` scope on the key. `client.Automations` manages the automations themselves (`List`, `Get`, `Create`, `Update`, `Delete`) and reads their activity (`ListEvents`, `automations:read`). On `Update`, a `*CommentDMAutomationInput` replaces the post's automation and `omnisocials.Null` removes it. Instagram rules: one private reply per comment, within 7 days; OmniSocials caps sends at 180 DMs an hour per account.

```go
post, err := client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Comment GUIDE and I will send you the link",
	Channels:    []string{"instagram"},
	ScheduledAt: "2026-08-01T09:00:00Z",
	MediaURLs:   []string{"https://example.com/guide-cover.jpg"},
	CommentDMAutomation: &omnisocials.CommentDMAutomationInput{
		Trigger: &omnisocials.CommentDMAutomationTrigger{Keywords: []string{"guide"}, Match: "word"},
		Actions: omnisocials.AutomationActions{
			DM: omnisocials.AutomationDM{Text: "Here is the guide: https://example.com/guide"},
			PublicReply: &omnisocials.AutomationPublicReply{
				Enabled:    omnisocials.Bool(true),
				Variations: []string{"Sent you a DM!", "Check your inbox"},
			},
		},
		Limits: &omnisocials.AutomationLimits{MaxSends: omnisocials.Int(500)},
	},
})
automationID := post.Data.CommentDMAutomation.ID

// Later: who was answered, and why someone was skipped
events, err := client.Automations.ListEvents(ctx, automationID, &omnisocials.AutomationEventListParams{Outcome: "sent"})
fmt.Println(events.Counts) // map[sent:12 skipped:3]
for _, event := range events.Data {
	fmt.Println(event.Kind, event.Outcome)
}
```

The events response is `Data`, `Counts` and `Pagination`, not the plain list: `Counts` is the number of events per outcome (`sent`, `skipped`, `failed`, `pending`, `queued`, `opted_out`, `waiting`). An automation can also listen on a post published outside OmniSocials (`client.Automations.Create` with `InstagramMediaID`), on the next post the workspace publishes (`Trigger: &omnisocials.AutomationTriggerInput{Scope: "next_post"}`) or on every post (`Scope: "any"`), and on DMs and story replies (`Kinds: []string{"dm", "story_reply"}`).

Follow first (`Actions.FollowGate`, Instagram only) sends the link only to people who follow the account. On a comment the private reply is a first message with one button; a tap makes OmniSocials ask Instagram whether the person follows. A follower gets `Actions.DM.Text`, anyone else gets the `NotFollowing` message with its button, and every later tap checks again. A DM or a story reply is checked at once. When Instagram gives no answer the link goes out anyway. Texts hold 640 characters and buttons 20; leave them empty for the defaults. A person who has not followed yet is an event with `Outcome` `waiting` and `event.FollowGate` (`State`, `Taps`, `Followed`, `Checked`, `AfterAsk`); `Stats.Followed` counts the people who followed after the ask. `Enabled: omnisocials.Bool(true)` on a Facebook or X automation is a `400 follow_gate_instagram_only`.

```go
_, err = client.Automations.Update(ctx, automationID, &omnisocials.AutomationUpdateParams{
	Actions: &omnisocials.AutomationActions{
		DM: omnisocials.AutomationDM{Text: "Here is the guide: https://example.com/guide"},
		FollowGate: &omnisocials.AutomationFollowGate{
			Enabled:      omnisocials.Bool(true),
			FirstMessage: &omnisocials.FollowGateMessage{Text: "Tap the button and I'll send you the link 👇", Button: "Send me the link"},
			NotFollowing: &omnisocials.FollowGateMessage{Text: "Almost there! Follow @yourbrand first, then tap the button again.", Button: "I follow now"},
		},
	},
})
```

A Facebook Page works the same way, and the DM lands in Messenger: set `Platform: "facebook"` on `AutomationCreateParams`, or `FacebookPostID` for a Page post published outside OmniSocials. On a post, `CommentDMAutomationInput{Platforms: []string{"instagram", "facebook"}, ...}` makes one automation per channel with the same keywords and DM. Facebook has no story replies.

On X, set `Platform: "x"` with `Trigger: &omnisocials.AutomationTriggerInput{Kinds: []string{"dm"}, Keywords: []string{"guide"}}`: the X account answers a keyword in a DM, once per person, and every answer ends with "Reply STOP to stop automatic messages." An active X automation needs X DMs on for the workspace (`409 x_dms_off`; `Status: "paused"` always saves). An answer over the send cap waits in a queue (`queued`) for up to an hour.

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

### Telegram channels and groups

`telegram` is a channel or group the workspace connected in OmniSocials; the OmniSocials bot posts to it. The `telegram` options: `silent`, `link_preview`, `pin`, `protect_content`, `spoiler` and `caption_above_media` (booleans), up to 3 `buttons` (`{ text, url }`, not on an album or a poll), a native `poll` (2 to 12 options of up to 100 characters, `open_period` 5 to 2628000 seconds, `multiple`; the post text is the question, max 300 characters, no media) and `first_comment` (a reply under the post). Text up to 4,096 characters; with media, text over 1,024 characters is sent as a reply under the media. Albums hold up to 10 photos and videos, files up to 50 MB, no Stories. Once sent, the post's `telegram.result` carries the message ids and the link. Telegram gives bots no view counts: analytics carry the reactions and the poll votes.

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "The new collection is live. Tap the button to shop.",
	Channels:    []string{"telegram"},
	ScheduledAt: "2026-11-02T09:00:00Z",
	MediaURLs:   []string{"https://example.com/collection.jpg"},
	Telegram: &omnisocials.TelegramPostOptions{
		Silent:       omnisocials.Bool(true),
		Pin:          omnisocials.Bool(true),
		Buttons:      []omnisocials.TelegramButton{{Text: "Shop now", URL: "https://example.com/shop"}},
		FirstComment: "Questions? Reply to this message.",
	},
})

// A native poll: the text is the question.
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Which day suits the live Q&A?",
	Channels: []string{"telegram"},
	Telegram: &omnisocials.TelegramPostOptions{
		Poll: &omnisocials.TelegramPoll{Options: []string{"Tuesday", "Thursday", "Saturday"}, OpenPeriod: 86400},
	},
})
```

On update, `TelegramPostOptionsUpdate` merges into the stored options; `Poll: omnisocials.Null` removes the poll.

### Discord channels

`discord` is one Discord channel the workspace connected in OmniSocials; the post goes out through the channel's incoming webhook and shows the webhook's name and picture. The `discord` options: `silent`, `link_preview` (default on), `allow_mentions` (default off, so @everyone, @here, role and user mentions show as text and ping nobody) and `spoiler` (booleans), `thread_name` (the post title in a forum or media channel, 1 to 100 characters), up to 5 `buttons` (`{ text, url }`, one row, not with a poll) and a native `poll` (2 to 10 options of up to 55 characters, `duration_hours` 1 to 768, default 24, `multiple`; the post text is the question, max 300 characters, no media). Text up to 2,000 characters. Up to 10 images, GIFs and videos per message; a video or GIF over Discord's 20 MB attachment limit is sent as a link in the text. No Stories. Once sent, the post's `discord.result` carries the message id and the link. Discord gives webhooks no view counts: analytics carry the reactions and the poll votes.

```go
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:     "Patch 2.4 is out. Read the notes below.",
	Channels:    []string{"discord"},
	ScheduledAt: "2026-11-02T09:00:00Z",
	MediaURLs:   []string{"https://example.com/patch-notes.png"},
	Discord: &omnisocials.DiscordPostOptions{
		Silent:  omnisocials.Bool(true),
		Buttons: []omnisocials.DiscordButton{{Text: "Patch notes", URL: "https://example.com/patch-2-4"}},
	},
})

// A native poll: the text is the question.
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Which mode do we test next?",
	Channels: []string{"discord"},
	Discord: &omnisocials.DiscordPostOptions{
		Poll: &omnisocials.DiscordPoll{Options: []string{"Ranked", "Co-op", "Arena"}, DurationHours: 48},
	},
})
```

On update, a `*omnisocials.DiscordPostOptionsUpdate` merges into the stored options (`Poll: omnisocials.Null` removes the poll) and `Discord: omnisocials.Null` removes every Discord option.

### Chained threads (X, Bluesky, Mastodon, Threads)

Provide 2 to 25 `ThreadParts` to publish a chained thread instead of a single tweet. Each part is capped at 280 characters (25,000 for X Premium/Premium+ accounts) and can carry its own media. The same `ThreadPart` shape works for `Bluesky` (300 chars per part), `Mastodon` (500 chars per part) and `Threads` (Meta Threads: 2 to 25 parts, 500 characters per part, up to 10 media per part; parts after the first publish as replies to the previous part, and the Threads caption is taken from part 1).

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

Threads posts can also carry a location tag: set `Threads.LocationID`, a Threads location id from `client.Locations.SearchThreads` (Instagram Place ids are not interchangeable). On a multi-part thread the tag lands on part 1; on update, set `LocationID: omnisocials.Null` to clear it. Threads location tagging is currently rolling out; until Meta approves the permissions it is disabled on production and calls return a clear error.

Threads posts can carry a topic too: set `Threads.TopicTag`, for example `"Build in public"`. Unlike a #tag in the text, a topic can hold spaces. 1 to 50 characters, no period and no ampersand; a leading `#` is dropped. On a multi-part thread the topic lands on part 1; on update, set `TopicTag: omnisocials.Null` to remove it.

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

From `EnforceFrom` (2026-08-14) the balance is checked at publish time, but credits are only deducted after the post successfully publishes (a failed publish is never charged). If the balance can't cover it, only the X target fails (other platforms publish normally); top up in the dashboard under Settings -> Organisation -> Billing -> Credits, then `Posts.Retry`. Posts without links and media on X stay free. X statistics use credits too, at X's price (see [X statistics use credits](#x-statistics-use-credits)). `client.Credits.Get(ctx)` reads the balance; top-ups are made in the dashboard.

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

These calls return a 409 `*APIError` and change nothing when the post is not in a state for them:

- `post_publishing`: `Update` or `Delete` while the post is publishing (status `posting`) or a retry publishes one of its channels. Poll `Get` until the status is `published`, `warning` or `failed`. An `Update` with only `Tags` still works.
- `review_required`: changes were requested on the post (`ApprovalStatus` `rejected`), so `Publish`, or an `Update` that would schedule it, is refused. It needs a new review: create it again with `ApprovalWorkflowID`. Saving it as a draft works.
- `retry_in_progress`: `Retry` while another retry of the post runs.
- `media_processing`: a media id in `Create`, `CreateAndPublish` or `Update` is still processing. Wait until `client.Media.Get` says `ready`, then send the same request again. A file whose processing failed returns `400 invalid_media`: upload it again.

### Approve or reject a post

```go
// Read the step you review first, then name it on the decision.
review, err := client.Posts.GetApproval(ctx, one.Data.ID)
step := 0 // 0 leaves step_order out
if review.Data.CurrentStep != nil {
	step = *review.Data.CurrentStep
}

_, err = client.Posts.Approve(ctx, one.Data.ID, &omnisocials.PostApproveParams{StepOrder: step}) // approve the step you reviewed
// or: reject and stop the workflow (comment optional, pass nil for none)
// _, err = client.Posts.Reject(ctx, one.Data.ID, &omnisocials.PostRejectParams{Comment: "Wrong CTA link, please fix.", StepOrder: step})

var apiErr *omnisocials.APIError
if errors.As(err, &apiErr) && apiErr.Code == "step_changed" {
	// Somebody decided since you read it: nothing was decided. Read the approval again.
	body, _ := apiErr.Body.(map[string]any)
	errObj, _ := body["error"].(map[string]any)
	log.Println("The review is on step", errObj["current_step_order"])
}
```

Only works on a post with `ApprovalStatus: "pending"` (`Status: "in_approval"`). Both act on behalf of the user who owns the API key, who must be a listed approver for the workflow's CURRENT step. Steps approve in order, so being an approver on a later step is not enough yet (returns a 403 `forbidden` error). Approving the last step finalizes the post (`scheduled` or `posting`); rejecting stops the whole workflow immediately, not just the current step.

`StepOrder` is optional: the `CurrentStep` of `GetApproval`. When the review is on another step now (somebody decided since you read it, or a retried call already went through), nothing is decided and the call returns a 409 `*APIError` with code `step_changed` and `error.current_step_order` in its `Body`, the step the review is on now. Without it the call acts on whatever step is current, so a retry after a timeout could approve the next step too. `client.Posts.Approve(ctx, id)` without params still works.

### Read the approval review

```go
review, err := client.Posts.GetApproval(ctx, one.Data.ID)
if err != nil {
	log.Fatal(err)
}
if rej := review.Data.Rejection; rej != nil && rej.Reason != nil {
	fmt.Println("Rejected by", rej.By.ID, "because:", *rej.Reason)
}
for _, step := range review.Data.Steps {
	fmt.Println(step.Order, step.Name, step.Status, len(step.Approvers))
}
```

`GetApproval` returns the review of a post that went through an approval workflow: `Status` (`none`, `pending`, `approved`, `rejected`), the `Workflow`, who requested it and when, `CurrentStep` (the step the post waits on, nil when the review ended), every step with its approvers and their decisions, the `Rejection` (`By`, `Reason`, `At`, `Step`; nil when nobody rejected) and the `Comments` thread, oldest first. Nullable fields are pointers. A post without an approval workflow returns `Status: "none"` with empty `Steps` and `Comments`. Read-only; needs the `posts:read` scope.

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

Videos over 100MB are processed in the background and come back with status `"processing"`; when such a file cannot be queued, the call returns a 503 `processing_unavailable` error (send the same request again in a minute). Every upload response includes a `Compatibility` block listing connected platforms that would reject the file.

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

Uploading a PDF rasterizes it into one image slide per page (max 20). The response carries `Slides` and `MediaIDs` alongside `Data` (the first slide). Pass ALL of `MediaIDs`, in order, to `Posts.Create` to post the deck as a carousel (on LinkedIn a native swipeable document made from the original PDF file, which is kept so text and links stay intact; an image carousel elsewhere). To keep the PDF as ONE library item instead of one item per page, pass `pdf_mode` = `"document"`: the response then has a single `data` item of type `document` (its page images in `pdf.pages`) and `media_ids` holds that one id, which expands into every page at post time.

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

## Post tags

Tags are your own labels for posts, such as a hook style or a format, to filter and compare posts in Analytics. They never go to a platform. Uses the `posts:read` / `posts:write` scopes (`Analytics.Tags` needs `analytics:read`).

```go
tag, err := client.Tags.Create(ctx, &omnisocials.TagCreateParams{Name: "Question hook", Color: "blue"}) // Color is optional
tags, err := client.Tags.List(ctx)
_, err = client.Tags.Update(ctx, tag.Data.ID, &omnisocials.TagUpdateParams{Name: "Question"})
deleted, err := client.Tags.Delete(ctx, tag.Data.ID) // takes it off every post; posts stay
fmt.Println(deleted.Data.PostsUntagged)

// Tag a post at create time: tag ids or names; a new name becomes a tag (at most 20 per post)
_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
	Content:  "Would you post this?",
	Channels: []string{"tiktok"},
	Tags:     []any{"Question", 14},
})

// Replace a post's tags; omnisocials.Null removes them all. Works on published posts too.
_, err = client.Posts.Update(ctx, "post_id", &omnisocials.PostUpdateParams{Tags: []any{"Question"}})

// Up to 500 posts at once: Add, Remove, or Set (replaces; []any{} removes every tag)
res, err := client.Tags.Apply(ctx, &omnisocials.TagApplyParams{
	PostIDs: []string{"1024", "1025"},
	Add:     []any{"Question"},
})
fmt.Println(res.Data.Updated, res.Data.NotFound)

// Filter: a tag id or name, comma separated for any of several
posts, err := client.Posts.List(ctx, &omnisocials.PostListParams{Tag: "Question"})
overview, err := client.Analytics.Overview(ctx, &omnisocials.AnalyticsOverviewParams{Period: "30d", Tag: "Question"})

// Compare tags against your normal (default: the last 90 days)
cmp, err := client.Analytics.Tags(ctx, &omnisocials.TagAnalyticsParams{StartDate: "2026-08-01", Platform: "tiktok"})
for _, row := range cmp.Data.Tags {
	if row.VsNormal != nil {
		fmt.Printf("%s: %+.0f%% vs normal (enough posts: %v)\n", row.Tag.Name, *row.VsNormal*100, row.EnoughPosts)
	}
}
```

`VsNormal` is a fraction (0.38 = 38% above your normal): each post is measured against the median impressions of its own channel in the period. Read it as a hint only when `EnoughPosts` is false. Every `Post` carries `Tags` and `TrialReel` (`{Strategy: "MANUAL" | "SS_PERFORMANCE"}` on an Instagram Trial Reel, else nil), and so does every `Analytics.Posts` entry.

Errors: `404 tag_not_found`; `409 tag_exists` (the existing tag is in the error body under `error.tag`), `tag_limit_reached` (200 tags per workspace); `400 invalid_tag_name` (1 to 40 characters), `invalid_tag_color`, `invalid_tags`, `invalid_post_ids`, `too_many_posts`, `too_many_tags`.

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

`Status` is `active` or `needs_reconnect` (the token was revoked or expired; Bluesky also when its sign-in could not be renewed). Posts to a `needs_reconnect` account fail until it is reconnected in the OmniSocials app. OmniSocials Connect customer workspaces can also show `pick_pending` (Facebook, Connect customers only: no Page chosen yet; `PickPending` is true then).

An X account also carries `XStatistics` (`x_statistics` on the wire: `posts`, `followers`, `post_schedule`, `source`, `billing`, `state`, `paused_reason`): how often OmniSocials reads its X statistics and who pays. `client.Accounts.Update(ctx, id, &omnisocials.AccountUpdateParams{...})` changes it; see [X statistics use credits](#x-statistics-use-credits).

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

The response is not wrapped in `data`. `platform`, `timezone`, `basis`, `grid` and `recommendations` sit at the top level.

```go
best, err := client.Analytics.BestTimes(ctx, &omnisocials.BestTimesParams{
	Platform: "instagram",
	Timezone: "Europe/Amsterdam",
})
slots, _ := best["recommendations"].([]any)
for _, slot := range slots {
	fmt.Println(slot)
}
```

### X statistics use credits

X bills every statistics read, and OmniSocials passes X's price on as credits: half a credit per post check (plus half a credit when the tweet has a video or GIF; each part of a thread counts) and 1 credit per follower count. Every X account carries `x_statistics`:

- `posts`: how often each post is checked. `full` (5 checks: about 5 hours, day 1, 3 and 7, and day 30 for posts with 100+ views), `light` (2 checks: about 5 hours and day 7) or `off`.
- `followers`: how often the follower count is read. `daily`, `weekly` or `off`.
- `source` (`account` or `default`), `billing` (`credits`), `state` (`active`, `paused`, `off`) and `paused_reason` (`insufficient_credits` or null).

When the balance reaches 0 the reads pause (`state` `paused`) and start again by themselves after a top-up; the webhook events `x_statistics.paused` and `x_statistics.resumed` say so. `x_statistics.charging` in the answer of `client.Credits.Get(ctx)` says whether reads are charged yet and from when.

```go
// How often an X account is checked (opt-in scope accounts:write, billing rights)
account, err := client.Accounts.Update(ctx, "1234_x", &omnisocials.AccountUpdateParams{
	XStatistics: &omnisocials.XStatisticsInput{
		Posts:     omnisocials.XStatisticsPostsLight, // one field or both
		Followers: omnisocials.XStatisticsFollowersDaily,
	},
})
if err == nil && account.Data.XStatistics != nil {
	fmt.Println(account.Data.XStatistics.State)
}

// "Update now": read one post's X numbers now (scope analytics:read)
fresh, err := client.Analytics.RefreshPost(ctx, "9123")
if err == nil {
	fmt.Println(fresh.Data.CreditsUsed, len(fresh.Data.Tweets))
}

// The balance and what X statistics cost (opt-in scope credits:read, billing rights)
credits, err := client.Credits.Get(ctx)
if err == nil {
	fmt.Println(credits.Data.Balance, credits.Data.XStatistics.TotalCreditsMonth)
}
```

`Analytics.RefreshPost` costs nothing when the post was already read that day (UTC) and works at most 5 times a post and 100 times an organisation a day. Errors: `402 insufficient_credits`, `409 refresh_not_available` (no published X tweet), `429 refresh_limit`, `502 refresh_failed` (nothing charged). Because it can cost credits, the SDK retries it only after a 429, never after a 5xx or a timeout. `Accounts.Update` needs the opt-in scope `accounts:write` and billing rights (manage_billing) for the key's creator, sends only the keys you give, and answers `400 validation_error`, `invalid_value` or `not_supported` (not an X account) and `403 insufficient_permission` (no billing rights for the key's creator). `Credits.Get` needs the opt-in scope `credits:read` and billing rights. The value constants are `XStatisticsPostsFull`, `XStatisticsPostsLight`, `XStatisticsPostsCustom`, `XStatisticsOff`, `XStatisticsFollowersDaily`, `XStatisticsFollowersWeekly`, `XStatisticsFollowersHourly` and `XStatisticsFollowers6h`.

## Locations (Instagram and Threads location tagging)

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

Threads locations use their own ids (a Facebook Place id is not a Threads location id) and a different response shape: `Locations` on success, `Error` (`{code, message}`) when unavailable. Search by text, or by coordinates instead of `Q`, and pass a result's `ID` as `ThreadsPostOptions.LocationID`. Threads location tagging is currently rolling out; until Meta approves the permissions it is disabled on production and calls return a clear error.

```go
spots, err := client.Locations.SearchThreads(ctx, &omnisocials.ThreadsLocationSearchParams{Q: "Griffith Observatory"})
// or around a point:
// &omnisocials.ThreadsLocationSearchParams{Latitude: omnisocials.Float64(34.1184), Longitude: omnisocials.Float64(-118.3004)}

if spots.Error == nil && len(spots.Locations) > 0 {
	_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
		Content:   "Golden hour at the observatory",
		Channels:  []string{"threads"},
		MediaURLs: []string{"https://example.com/observatory.jpg"},
		Threads:   &omnisocials.ThreadsPostOptions{LocationID: spots.Locations[0].ID},
	})
}
```

## Pinterest product tags

Tag products on a Pin so people can shop the items in the image. `client.Pinterest.ListProducts` returns the product Pins of the connected Pinterest account; pass their `PinID` values (max 24) as `product_tags` in the `Pinterest` options of the post. Only product Pins of your own account can be tagged; products of other merchants cannot. The tags are added right after the Pin is published. A product that Pinterest refuses never fails the post: the outcome is on the post as `Pinterest["product_tags_result"]` (`requested`, `tagged`, `skipped`, `error`).

```go
list, err := client.Pinterest.ListProducts(ctx, nil)
if err != nil {
	log.Fatal(err)
}

if list.Error != nil {
	// HTTP 200 without products: pinterest_not_connected,
	// pinterest_catalog_access_required or platform_error
	log.Println(list.Error.Code, list.Error.Message)
} else {
	productTags := []string{}
	for _, product := range list.Products {
		if len(productTags) < 3 {
			productTags = append(productTags, product.PinID)
		}
	}

	_, err = client.Posts.Create(ctx, &omnisocials.PostCreateParams{
		Content:     "Our summer picks",
		Channels:    []string{"pinterest"},
		MediaURLs:   []string{"https://example.com/summer-look.jpg"},
		ScheduledAt: "2026-08-01T09:00:00Z",
		Pinterest: map[string]any{
			"board_id":     "1234567890",
			"title":        "Summer picks",
			"product_tags": productTags,
		},
	})
}
```

With nil params (or an empty `Source`) the list reads the Pinterest catalog (with `Price`, `Currency`, `Availability` and `ItemID`) when the connection has catalog access, else the account's own Pins. Catalog access is given one time in the OmniSocials composer: Pinterest options, Add products, Connect catalog. `Source: "pins"` scans up to 250 Pins per call, so `Products` can be empty while `Bookmark` is set; call again with `&omnisocials.PinterestProductListParams{Bookmark: *list.Bookmark}`. To check one Pin id or Pin link before you post, call `client.Pinterest.ValidateProduct(ctx, "813744226420795884")`. On `Posts.Update` the `Pinterest` map replaces the stored one, so leave `product_tags` out to remove the tags.

## Ads

Boost a published post or create a new ad on Meta, Pinterest, TikTok, LinkedIn, X and Google. Boosts and new ads are drafts: your code writes the draft, and the person who pays sees the amount and starts the ad in OmniSocials from the draft's `ReviewURL`. A key starts and raises spend itself only with the opt-in **Start ads** switch (scope `ads:spend`). Without it `Ads.Activate`, `Ads.ActivateMulti`, `Ads.Resume`, `Ads.Drafts.Launch`, a budget raise or any change of a campaign that reached the network through `Ads.Update`, and the parity calls that switch something on or raise a budget answer `403 start_in_app` (a `*PermissionDeniedError`), and the error message names the page where the person does it. Google (`APIDraftOnly` of `Ads.Capabilities`) always stops at the draft, also with `ads:spend`. `Ads.Pause` and `Ads.Delete` stay open, except on the networks in `APIStopInApp` (Google), where a campaign that reached the network also answers `403 start_in_app`. Only a paused campaign resumes (`409 not_paused`). The calls that start or raise spend repeat the amount (`ConfirmBudgetAmount`) and are not retried after a 5xx or a connection error, only after a 429.

The `ads:read` and `ads:write` scopes are opt-in: create the API key with **Ads** switched on (Settings, API). A key without them gets a `*PermissionDeniedError` with code `insufficient_scope`. `ads:spend` is the **Start ads** switch under it, offered only to a person with `manage_ads`. Ad accounts are connected and picked in the OmniSocials dashboard. Budgets are in major units (10 = 10 EUR) in the ad account's currency: a draft whose `Budget.Currency` differs from the attached ad account's currency answers `400 currency_mismatch`. Ads errors may carry `param`, the field at fault: read it from `APIError.Body` (`error.param`). Campaign and ad account ids are `omnisocials.AdID` values; the API may send them as numbers or as strings, and both decode.

### Create an ad (a draft the person starts)

```go
caps, err := client.Ads.Capabilities(ctx)
fmt.Println(caps.Data.CreateNetworks, caps.Data.NetworkStates)

draft, err := client.Ads.Drafts.Create(ctx, &omnisocials.AdDraftParams{
	Name:      "Autumn sale",
	Objective: "traffic",
	Platforms: []string{"meta", "linkedin"},
	Budget:    omnisocials.AdBudget{Amount: 10, Currency: "EUR"},
	Schedule:  &omnisocials.AdDraftSchedule{DurationDays: omnisocials.Int(7)},
	Targeting: omnisocials.AdDraftTargeting{
		Countries:         []string{"NL", "BE"},
		AutomaticAudience: omnisocials.Bool(true),
	},
	Ads: []omnisocials.AdDraftAd{{
		Media:       []omnisocials.AdMediaFile{{URL: "https://example.com/sale.jpg", Type: "image", Width: 1080, Height: 1350}},
		PrimaryText: "30% off everything this week",
		Headline:    "Autumn sale",
		CTA:         "shop_now",
		Link:        "https://example.com/sale",
	}},
})
if err != nil {
	log.Fatal(err)
}

// A draft spends nothing. The person checks it and starts it in OmniSocials:
fmt.Println("Check and start your ad:", draft.Data.ReviewURL)
```

With a key that has `ads:spend`, `client.Ads.Drafts.Launch(ctx, draft.Data.GroupID, &omnisocials.AdDraftLaunchParams{ConfirmBudgetAmount: 10})` starts the draft after the person agreed to the amount; Google is skipped with code `start_in_app`. Without `ads:spend` the call answers `403 start_in_app`.

A draft needs no ad account; `client.Ads.Drafts.AttachAccount(ctx, groupID, &omnisocials.AdDraftAccountParams{Platform: "meta", AdAccountID: id})` sets one, and `Campaigns[].Readiness` says what still stops a network. `Ads.Drafts.Update` replaces the draft with the full body (`PUT`). On Meta an ad can carry a second upright 9:16 file for Stories and Reels: `PlacementMedia: &omnisocials.AdPlacementMedia{StoriesReels: &omnisocials.AdMediaFile{URL: "...", Type: "image"}}`.

### Boost a post

```go
accounts, err := client.Ads.ListAccounts(ctx)
var meta *omnisocials.AdAccount
for i, a := range accounts.Data {
	if a.Platform == "meta" && a.IsSelected {
		meta = &accounts.Data[i]
		break
	}
}

currency := "EUR"
if meta.AccountInfo != nil && meta.AccountInfo.Currency != "" {
	currency = meta.AccountInfo.Currency
}

boost, err := client.Ads.Boost(ctx, &omnisocials.AdBoostParams{
	PostID:      "12345",
	AdAccountID: meta.ID,
	Budget:      omnisocials.AdBudget{Type: "daily", Amount: 5, Currency: currency},
	Targeting:   map[string]any{"countries": []string{"NL"}},
})

// The boost is a draft: the person starts it in OmniSocials.
fmt.Println(boost.Data.Status) // "draft"
```

`client.Ads.BoostMulti` makes one draft boost per ad account (up to 10, the same budget each). `client.Ads.Suggestions` lists posts that do better than your median, `client.Ads.Overview` the totals (`Totals.Spend` is nil when the campaigns use more than one currency: read `TotalsByCurrency`), and `client.Ads.List`, `Get`, `Pause` and `Delete` manage campaigns. Keyword ideas through the connected Google Ads account: `client.Ads.ResearchKeywords(ctx, &omnisocials.AdKeywordResearchParams{Keywords: []string{"running shoes"}, Country: "NL"})` and `client.Ads.KeywordMetrics(ctx, &omnisocials.AdKeywordMetricsParams{Keywords: ...})`.

### More of the Ads API

Sub-services cover the rest of the Ads API: `Objects` (the tree of an ad account, also campaigns made outside OmniSocials, and their import), `CampaignAds`, `Comments`, `Targeting`, `Audiences`, `LeadForms`, `Leads`, `Conversions`, `Insights`, `Reports`, `Rules`, `Results`, `Media`, `Previews`, `Partnership`, `Accounts` (health, finance, settings, people), `ReachFrequency`, `Studies`, `BudgetSchedules` and `Google`, plus `Ads.BulkStatus` and `Ads.Duplicate`. A network without a feature answers `400 not_supported_on_network`; `Capabilities` lists `Features` and `TargetingTypes` per network. Every call that can start or raise spend takes the person's own amount and is not retried after a 5xx or a connection error: `Objects.Update`, `Objects.SetStatus`, `BulkStatus`, `CampaignAds.SetStatus`, `Rules.Create`, `Rules.Update`, `Rules.Undo`, `Accounts.UpdateSettings`, `ReachFrequency.Reserve` and `BudgetSchedules.Create`. Switching on, a budget or spend cap raise, a change of a campaign on the network (also `Objects.Duplicate`, `CampaignAds.Add` and `Studies.Create`) need a key with `ads:spend`: without it, and on Google always, they answer `403 start_in_app` with the page where a person does it in the app. A rule that switches on or raises a budget needs `ads:spend` too (else `403 rule_raises_spend`); a rule that notifies, pauses or lowers a budget works with `ads:write`, and no rule acts on Google. Pausing, deleting and archiving stay open, except on Google.

```go
interests, err := client.Ads.Targeting.Search(ctx, &omnisocials.AdTargetingSearchParams{
	AdAccountID: meta.ID, Type: "interest", Q: "running",
})
// Use the ids in AdDraftTargeting.Networks["meta"].Include["interest"].
```

## Webhooks

Events: `post.scheduled`, `post.published`, `post.failed`, `post.approved` (the last step of a post's approval workflow is approved) and `post.rejected` (an approver rejects the post; it will not publish). The two approval events carry `data.approval` with `status`, `decided_by` (the approver's user id) and `reason` (null on `post.approved`), and an empty `data.targets`. `ad.status_changed` fires when an ad campaign changes status (for example `pending_review` to `active` or `rejected`); its `data` is the campaign, not a post: decode it into `omnisocials.AdStatusChangedEventData` (`CampaignID`, `Name`, `Platform`, `CampaignType`, `Status`, `PreviousStatus`, `ExternalCampaignID`, `Budget`). The other ads events are listed in `omnisocials.AdsWebhookEvents` (`ad.created`, `ad.updated`, `ad.review`, `ad.delivery_issue`, `campaign.created`, `campaign.updated`, `campaign.budget`, `lead.received`, `ads.sync_failed`, `ads.sync_recovered`, `ads.initial_sync_completed`, `ad_rule.triggered`). Subscribing to `ad.status_changed` or any other ads event needs the `ads:read` scope on the key and `access_ads` for its person (`403 insufficient_scope` / `insufficient_permission`). `x_statistics.paused` and `x_statistics.resumed` fire when the organisation's X statistics stop at 0 credits or start again after a top-up, one delivery per workspace with X; `data` is `{ workspace_id, account_id, reason }` (`reason` is `insufficient_credits` or `credits_added`). `XStatisticsWebhookEvents` lists them and `XStatisticsEventData` types the `data`.

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
fmt.Println(rotated.Data.Secret) // the old secret keeps signing for 24 hours
err = client.Webhooks.Delete(ctx, webhook.Data.ID)
```

Every webhook carries `WorkspaceID`. `List` and `Get` also return the organisation-wide webhooks of the key's organisation (`WorkspaceID` nil, made in the OmniSocials dashboard; they receive the events of every workspace), and `Update`, `Delete` and `RotateSecret` work on them too. Webhooks you create through the API belong to the key's workspace. When the person behind a webhook loses access to the organisation or workspace, OmniSocials switches the webhook off: `IsActive` is false and `LastFailure.Error` says why. `Update` with `IsActive: omnisocials.Bool(true)` switches it on again.

### Verify deliveries (net/http example)

Every delivery is signed with your webhook secret. The `X-OmniSocials-Signature` header has the form `t=<unix>,v1=<hex>` where the hex value is an HMAC-SHA256 of `"{timestamp}.{rawBody}"`. Always verify against the RAW request body:

```go
package main

import (
	"encoding/json"
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
		case "post.rejected":
			approval, _ := data["approval"].(map[string]any)
			log.Println("Rejected:", data["post_id"], approval["reason"])
		case "ad.status_changed":
			// The data is the campaign, not a post.
			var ad struct {
				Data omnisocials.AdStatusChangedEventData `json:"data"`
			}
			if err := json.Unmarshal(payload, &ad); err == nil {
				log.Println("Ad:", ad.Data.CampaignID, ad.Data.Status)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

`VerifyWebhookSignature` uses a constant-time comparison, rejects timestamps older than the tolerance (replay protection), returns a `*WebhookVerificationError` on any failure, and returns the parsed event on success.

### Rotate the secret without missing deliveries

For 24 hours after `RotateSecret`, every delivery carries two headers: `X-OmniSocials-Signature`, signed with the new secret, and `X-OmniSocials-Signature-Previous`, signed with the old secret (same format, same timestamp and body). Verify the first with the new secret and, when that fails, the second with the old one:

```go
func verifyDelivery(payload []byte, h http.Header, newSecret, oldSecret string) (map[string]any, error) {
	event, err := omnisocials.VerifyWebhookSignature(payload, h.Get("X-OmniSocials-Signature"), newSecret, 0)
	if err == nil {
		return event, nil
	}
	if previous := h.Get("X-OmniSocials-Signature-Previous"); previous != "" {
		return omnisocials.VerifyWebhookSignature(payload, previous, oldSecret, 0)
	}
	return nil, err
}
```

Rotating again within the 24 hours replaces the previous secret with the one you just rotated out.

## Inbox

Social inbox conversations (DMs, comments, and mentions) across connected platforms: Instagram, Facebook, LinkedIn, TikTok (video comments only), YouTube (video comments only), X (DMs), and Threads (replies and mentions, no DMs). TikTok and YouTube replies are comments only; TikTok replies are capped at 150 characters. The Threads inbox needs a Threads connection with the reply permission: a Threads account connected before 2026-09-14 needs a one-time reconnect in the dashboard (until then replies and hides answer 401 `reauth_required`).

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

// Hide a comment someone left on one of your posts (Facebook, Instagram,
// TikTok, YouTube, Threads). false unhides.
_, err = client.Inbox.Hide(ctx, thread.Data[0].ID, true)

// Like a comment as the Page or account (Facebook, LinkedIn Page, TikTok).
// false unlikes. The returned message carries Liked.
liked, err := client.Inbox.Like(ctx, thread.Data[0].ID, true)
fmt.Println(*liked.Data.Liked) // true

// Delete a comment outright, or one of your own replies (Facebook, Instagram,
// TikTok; YouTube: hide instead). Replies under it go with it; their ids come
// back as RemovedReplyIDs.
deleted, err := client.Inbox.DeleteMessage(ctx, thread.Data[0].ID)
fmt.Println(deleted.Data.RemovedReplyIDs)
```

**Work queue: what needs an answer.** `Inbox.Next` hands out the next conversation that still needs a reply (the customer's latest DM with no reply after it, or an unreplied comment/mention that is not hidden), with the whole thread and the post it belongs to (`Post.URL`, `Post.MediaType`), so a reply can be drafted from one call. DMs that can still be answered come first, then Instagram/Facebook DMs whose 24-hour window has closed (`ReplyWindow.Open` is false: answer those from the native app or mark them read), then comments and mentions, oldest first. Replies typed in the native apps count as answers. Only unread items are served by default, so `MarkRead` is the durable way to skip one; `Exclude` skips conversation ids for the current session only. Always set `MessageID` to `Data.Message.ID` on `Reply` for comment threads: every comment on a post shares one conversation, and without it the reply goes under the newest comment on the post. Set `IncludeNext` on `Reply` to get the following item in the same response. `InboxListParams{Unanswered: omnisocials.Bool(true)}` gives the same set as a plain list.

```go
next, err := client.Inbox.Next(ctx, &omnisocials.InboxNextParams{Platform: "instagram"})
for err == nil && next.Data != nil {
	fmt.Printf("%d left. %s: %s\n", next.Remaining, next.Data.Message.Sender.Username, next.Data.Message.Text)

	if !next.Data.ReplyWindow.Open {
		// An Instagram/Facebook DM past Meta's 24-hour window: Reply would
		// answer 422 outside_messaging_window. Answer it in the app, or skip it.
		if _, err = client.Inbox.MarkRead(ctx, next.Data.Message.ConversationID); err == nil {
			next, err = client.Inbox.Next(ctx, &omnisocials.InboxNextParams{Platform: "instagram"})
		}
		continue
	}

	var reply *omnisocials.InboxReplyResponse
	reply, err = client.Inbox.Reply(ctx, next.Data.Message.ConversationID, &omnisocials.InboxReplyParams{
		Text:        "Thanks! DM sent.",
		MessageID:   next.Data.Message.ID, // the comment being answered, not the newest one
		IncludeNext: true,
	})
	if err == nil {
		next = &omnisocials.InboxNextResponse{Data: reply.Next}
		if reply.Remaining != nil {
			next.Remaining = *reply.Remaining
		}
	}
}
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
		log.Println("workspace's X inbox is suspended until a top-up")
	}
}
```

## Limits and feature requests

`client.Limits.List` lists what OmniSocials cannot do, why (`Says`) and what to do instead (`Instead`): platform rules, features not built yet, features that work in the OmniSocials app only. Read it before you tell a person that something is not possible. When the person still wants a `Requestable` entry, send a feature request with its `LimitID`. The request is stored for the person who owns the key; they see it in OmniSocials (Settings, Request a feature) and get an email when its status changes. Works with every key, no scope needed.

```go
limits, err := client.Limits.List(ctx, &omnisocials.LimitListParams{Platform: "telegram"}) // Area: "posts", "inbox", ...
if err != nil {
	log.Fatal(err)
}
for _, l := range limits.Data {
	fmt.Println(l.ID, l.Says, l.Requestable) // l.Instead is a *string
}

res, err := client.FeatureRequests.Create(ctx, &omnisocials.FeatureRequestCreateParams{
	Summary:  "Post our daily update to the Announcements topic of our Telegram group, not to the whole group.",
	Platform: "telegram",
	LimitID:  "telegram.forum_topics",
})
var apiErr *omnisocials.APIError
switch {
case errors.As(err, &apiErr) && apiErr.Code == "not_requestable":
	// A platform rule or a choice of OmniSocials: nothing was stored. The
	// message gives the reason and what to do instead (the entry itself is
	// in apiErr.Body under error.limit).
	fmt.Println(apiErr.Message)
case err != nil:
	log.Fatal(err)
case res.Outcome == omnisocials.FeatureRequestCreated:
	fmt.Println("sent, request", res.Data.ID)
default:
	// FeatureRequestVoted: a vote on your organisation's open request.
	// FeatureRequestAlreadyRequested: you asked before; res.Data is that request.
	fmt.Println(res.Outcome, res.Data.ID, res.Data.Status)
}

mine, err := client.FeatureRequests.List(ctx)                // newest first, at most 50
one, err := client.FeatureRequests.Get(ctx, mine.Data[0].ID) // Status, TeamAnswer, TeamReplies
```

A `Limit` has `ID`, `Kind` (one of `omnisocials.LimitKinds`: `platform`, `by_design`, `not_built`, `beta`, `dashboard_only`, `needs_setup`), `KindLabel`, `Area`, `Platforms`, `Title`, `Says`, `Instead` and `Requestable` (false for `platform` and `by_design`); `LimitsResponse.Kinds` holds a label per kind. A `FeatureRequest` carries `Status` (`open`, `in_progress`, `completed`, `declined`), `VotesCount`, `TeamAnswer` and `TeamReplies`. `Summary` is 10 to 1000 characters (what the person wants to do and why), `Title` at most 120 (made from the summary when empty); `Area` is one of `omnisocials.LimitAreas`. Errors: `400 invalid_summary` / `invalid_title` / `invalid_platform` / `invalid_area` / `unknown_limit`, `409 not_requestable`, `429 too_many_requests` (10 an hour per person, a `*RateLimitError`), and `404 not_found` on `Get`.

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

Each API key carries scopes: `posts:read`, `posts:write`, `media:write`, `accounts:read`, `analytics:read`, `webhooks:manage`, plus the opt-in `automations:read` / `automations:write` for comment-to-DM automations, `accounts:write` for `Accounts.Update` and `credits:read` for `Credits.Get`. A call with a missing scope returns a `*PermissionDeniedError` with code `insufficient_scope`.

## Documentation

Full API reference and guides: [https://docs.omnisocials.com](https://docs.omnisocials.com)

## License

MIT
