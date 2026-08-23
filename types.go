package omnisocials

import "encoding/json"

// Shared request/response types. Request field names are ported from the
// canonical MCP client (mcp-server/src/client.ts); response shapes follow the
// live API (backend/routes/api/v1). Deep platform-specific blobs stay as
// map[string]any passthrough on purpose.

// ---- Envelope ---------------------------------------------------------------

// Pagination describes a list response's paging block.
type Pagination struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// ItemResponse is the single-item envelope `{ "data": {...} }`. The body is
// returned as-is: the `data` field is not unwrapped.
type ItemResponse[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
	// Warnings are non-blocking notices, present only on post create
	// responses that trigger one (currently only x_url_post_credits — X's
	// link-post fee passed through as prepaid credits). Nil otherwise.
	Warnings []PostWarning `json:"warnings,omitempty"`
}

// PostWarning is one entry of the optional top-level `warnings` array on post
// create responses. Currently only code "x_url_post_credits": X bills API
// posts whose text contains a URL at a premium, and OmniSocials passes that
// through as prepaid credits (20 credits per URL-containing tweet; threads
// billed per part with a link). Debiting starts at EnforceFrom (2026-08-14);
// if the company balance can't cover it at publish time, only the X target
// fails and can be retried after topping up in the dashboard (Settings ->
// Organisation -> Billing -> Credits). Posts without links stay free.
type PostWarning struct {
	// Code is the warning identifier, e.g. "x_url_post_credits".
	Code string `json:"code"`
	// Message is a human-readable explanation.
	Message string `json:"message"`
	// CreditsRequired is the credits this post will use when it publishes.
	CreditsRequired int `json:"credits_required,omitempty"`
	// CreditsBalance is the company's current credit balance (nil if
	// unavailable).
	CreditsBalance *int `json:"credits_balance,omitempty"`
	// Enforced reports whether the credit debit is enforced yet
	// (warning-only before EnforceFrom).
	Enforced bool `json:"enforced,omitempty"`
	// EnforceFrom is the ISO date enforcement starts, e.g. "2026-08-14".
	EnforceFrom string `json:"enforce_from,omitempty"`
}

// ListResponse is the list envelope `{ "data": [...], "pagination": {...} }`.
type ListResponse[T any] struct {
	Data       []T         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// ---- Nullable field helper --------------------------------------------------

// Null is an explicit JSON null for update fields that distinguish "omitted"
// from "cleared". Assign it to fields typed `any` that document Null support,
// e.g. MediaUpdateParams.FolderID (move to root), FolderUpdateParams.ParentID
// (move to top level), or XPostOptionsUpdate.ThreadParts (clear thread mode).
var Null = json.RawMessage("null")

// Bool returns a pointer to v, for optional boolean params.
func Bool(v bool) *bool { return &v }

// String returns a pointer to v, for optional string params (including
// explicit empty strings, e.g. clearing a media item's name).
func String(v string) *string { return &v }

// Int returns a pointer to v, for optional integer params.
func Int(v int) *int { return &v }

// Float64 returns a pointer to v, for optional float params.
func Float64(v float64) *float64 { return &v }

// ---- Media entries (alt text) -------------------------------------------------

// MediaURLEntry is one media_urls entry carrying per-media alt text
// (accessibility description, max 1500 chars). Alt text is delivered to
// Mastodon (media description), Bluesky (embed alt), X (photo/GIF media
// metadata), Pinterest (pin alt_text fallback), Instagram (images), and
// LinkedIn (images). Plain string entries and MediaURLEntry values can be
// mixed in the same []any slice.
type MediaURLEntry struct {
	// URL is the public media URL.
	URL string `json:"url"`
	// Alt is the accessibility description (max 1500 chars).
	Alt string `json:"alt,omitempty"`
}

// MediaIDEntry is one media_ids entry carrying per-media alt text. See
// MediaURLEntry for which platforms receive alt text.
type MediaIDEntry struct {
	// ID is the Media Library id.
	ID string `json:"id"`
	// Alt is the accessibility description (max 1500 chars).
	Alt string `json:"alt,omitempty"`
}

// ---- Thread parts (X / Bluesky / Mastodon / Threads) -------------------------

// ThreadPart is one segment of a thread on X (max 280 chars), Bluesky
// (max 300 graphemes), Mastodon (max 500 chars by default), or Threads
// (max 500 chars). Each part can carry up to 4 media items (MediaIDs +
// MediaURLs combined); Threads allows up to 10 per part, images and videos
// mixed.
type ThreadPart struct {
	// Text is the part's text.
	Text string `json:"text"`
	// MediaIDs are Library ids from a media upload: a []string, or a
	// []MediaIDEntry (or mixed []any) for per-media alt text.
	MediaIDs any `json:"media_ids,omitempty"`
	// MediaURLs are public external URLs: a []string, or a []MediaURLEntry
	// (or mixed []any) for per-media alt text.
	MediaURLs any `json:"media_urls,omitempty"`
}

// XPostOptions holds X (Twitter) specific options for post creation.
type XPostOptions struct {
	// ReplySettings: "" (everyone), "following", or "mentionedUsers".
	ReplySettings   string `json:"reply_settings,omitempty"`
	PaidPartnership *bool  `json:"paid_partnership,omitempty"`
	MadeWithAI      *bool  `json:"made_with_ai,omitempty"`
	// ThreadParts: provide 2-25 parts to publish as a thread. Omit for a
	// single tweet.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// XPostOptionsUpdate is the update-side variant of XPostOptions.
type XPostOptionsUpdate struct {
	ReplySettings   *string `json:"reply_settings,omitempty"`
	PaidPartnership *bool   `json:"paid_partnership,omitempty"`
	MadeWithAI      *bool   `json:"made_with_ai,omitempty"`
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single tweet).
	ThreadParts any `json:"thread_parts,omitempty"`
}

// BlueskyPostOptions holds Bluesky specific options for post creation.
type BlueskyPostOptions struct {
	// ThreadParts: provide 2-25 parts to publish as a thread. Omit for a
	// single post.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// BlueskyPostOptionsUpdate is the update-side variant of BlueskyPostOptions.
type BlueskyPostOptionsUpdate struct {
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single post).
	ThreadParts any `json:"thread_parts,omitempty"`
}

// MastodonPostOptions holds Mastodon specific options for post creation.
type MastodonPostOptions struct {
	// ThreadParts: provide 2-25 parts to publish as a thread. Omit for a
	// single status.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// MastodonPostOptionsUpdate is the update-side variant of MastodonPostOptions.
type MastodonPostOptionsUpdate struct {
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single status).
	ThreadParts any `json:"thread_parts,omitempty"`
}

// ThreadsPostOptions holds Threads (Meta) specific options for post creation.
type ThreadsPostOptions struct {
	// ThreadParts: provide 2-25 parts to publish as a chained thread; parts
	// after the first publish as replies to the previous part, and the
	// Threads caption is taken from part 1. Omit for a single post.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// ThreadsPostOptionsUpdate is the update-side variant of ThreadsPostOptions.
type ThreadsPostOptionsUpdate struct {
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single post).
	ThreadParts any `json:"thread_parts,omitempty"`
}

// ---- LinkedIn poll -----------------------------------------------------------

// LinkedInPoll is a non-sponsored LinkedIn poll (question + 2-4 options +
// duration). Mutually exclusive with media and a link share on that
// channel's post - a poll takes priority over both at publish time.
type LinkedInPoll struct {
	// Question is the poll question (max 140 characters).
	Question string `json:"question"`
	// Options are 2-4 answer options (max 30 characters each).
	Options []string `json:"options"`
	// Duration: "ONE_DAY", "THREE_DAYS", "SEVEN_DAYS", or "FOURTEEN_DAYS".
	Duration string `json:"duration"`
}

// LinkedInPolls holds a poll per LinkedIn channel, independent of each
// other - LinkedIn (personal profile) and LinkedInPage (company page) can
// each carry their own poll, or none.
type LinkedInPolls struct {
	LinkedIn     *LinkedInPoll `json:"linkedin,omitempty"`
	LinkedInPage *LinkedInPoll `json:"linkedin_page,omitempty"`
}

// ---- Misc shared types --------------------------------------------------------

// UserTag is an Instagram photo user tag: a username positioned at relative
// coordinates (0-1) on an image.
type UserTag struct {
	Username string  `json:"username"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	// ImageIndex targets one image of a carousel (0-based).
	ImageIndex *int `json:"image_index,omitempty"`
}

// HealthResponse is the `GET /health` response.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}
