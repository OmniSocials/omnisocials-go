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

// ---- Thread parts (X / Bluesky / Mastodon) ----------------------------------

// ThreadPart is one segment of a thread on X (max 280 chars), Bluesky
// (max 300 graphemes), or Mastodon (max 500 chars by default). Each part can
// carry up to 4 media items (MediaIDs + MediaURLs combined).
type ThreadPart struct {
	// Text is the part's text.
	Text string `json:"text"`
	// MediaIDs are Library ids from a media upload.
	MediaIDs []string `json:"media_ids,omitempty"`
	// MediaURLs are public external URLs.
	MediaURLs []string `json:"media_urls,omitempty"`
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
