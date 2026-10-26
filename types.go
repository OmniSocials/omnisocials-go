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
	// Warnings are non-blocking notices, present only on post create and
	// update responses that trigger one (x_url_post_credits — X's link-post
	// fee passed through as prepaid credits — or image_fit — a feed image
	// cropped or kept for a channel). Nil otherwise.
	Warnings []PostWarning `json:"warnings,omitempty"`
}

// PostWarning is one entry of the optional top-level `warnings` array on post
// create and update responses. Two codes. "x_url_post_credits": X bills API
// posts whose text contains a URL at a premium, and OmniSocials passes that
// through as prepaid credits (20 credits per URL-containing tweet; threads
// billed per part with a link). Debiting starts at EnforceFrom (2026-08-14);
// if the company balance can't cover it at publish time, only the X target
// fails and can be retried after topping up in the dashboard (Settings ->
// Organisation -> Billing -> Credits). Posts without links stay free.
// "image_fit": a feed image whose ratio is outside what a channel shows
// uncropped; Action says what was done for that channel (see
// PostCreateParams.MediaFit), with Platform, MediaURL, Ratio, Target and
// CropURL.
type PostWarning struct {
	// Code is the warning identifier: "x_url_post_credits" or "image_fit".
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
	// Platform is the channel an image_fit decision is for, e.g.
	// "instagram".
	Platform string `json:"platform,omitempty"`
	// MediaURL is the original image of an image_fit decision.
	MediaURL string `json:"media_url,omitempty"`
	// Ratio is the image's ratio, e.g. "9:16" (image_fit).
	Ratio string `json:"ratio,omitempty"`
	// Target is the channel's window, e.g. "4:5" (image_fit).
	Target string `json:"target,omitempty"`
	// Action is what happened for that channel: "cropped", "kept" or
	// "crop_failed" (image_fit).
	Action string `json:"action,omitempty"`
	// CropURL is the cropped file when Action is "cropped" (image_fit).
	CropURL string `json:"crop_url,omitempty"`
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

// ThreadPart is one segment of a thread on X (max 280 chars, 25,000 for
// Premium/Premium+ accounts), Bluesky (max 300 graphemes), Mastodon (max 500
// chars by default), or Threads (max 500 chars). Each part can carry up to 4
// media items (MediaIDs + MediaURLs combined); Threads allows up to 10 per
// part, images and videos mixed.
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

// ChannelPoll is a poll on an X, Threads or Mastodon post (the post text is
// the question). X: 2 to 4 options of up to 25 characters, DurationMinutes
// 5 to 10080 (default 1440). Threads: 2 to 4 options of 1 to 25 characters,
// closed by Threads after 24 hours. Mastodon: options and ExpiresIn (seconds)
// within the instance's limits, plus Multiple and HideTotals. A poll and
// media do not go together on these platforms; on a thread the poll goes on
// the first part.
type ChannelPoll struct {
	Options         []string `json:"options"`
	DurationMinutes int      `json:"duration_minutes,omitempty"`
	ExpiresIn       int      `json:"expires_in,omitempty"`
	Multiple        *bool    `json:"multiple,omitempty"`
	HideTotals      *bool    `json:"hide_totals,omitempty"`
}

// XPostOptions holds X (Twitter) specific options for post creation.
type XPostOptions struct {
	// ReplySettings: "" (everyone), "following", or "mentionedUsers".
	ReplySettings   string `json:"reply_settings,omitempty"`
	PaidPartnership *bool  `json:"paid_partnership,omitempty"`
	MadeWithAI      *bool  `json:"made_with_ai,omitempty"`
	// Poll on the tweet; see ChannelPoll.
	Poll *ChannelPoll `json:"poll,omitempty"`
	// ThreadParts: provide 2-25 parts to publish as a thread. Omit for a
	// single tweet.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// XPostOptionsUpdate is the update-side variant of XPostOptions.
type XPostOptionsUpdate struct {
	ReplySettings   *string `json:"reply_settings,omitempty"`
	PaidPartnership *bool   `json:"paid_partnership,omitempty"`
	MadeWithAI      *bool   `json:"made_with_ai,omitempty"`
	// Poll: leave nil to keep the existing poll untouched, set a
	// *ChannelPoll to replace it, or set omnisocials.Null to remove it.
	Poll any `json:"poll,omitempty"`
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
	// Poll on the status; see ChannelPoll.
	Poll *ChannelPoll `json:"poll,omitempty"`
	// ThreadParts: provide 2-25 parts to publish as a thread. Omit for a
	// single status.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
}

// MastodonPostOptionsUpdate is the update-side variant of MastodonPostOptions.
type MastodonPostOptionsUpdate struct {
	// Poll: leave nil to keep the existing poll untouched, set a
	// *ChannelPoll to replace it, or set omnisocials.Null to remove it.
	Poll any `json:"poll,omitempty"`
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single status).
	ThreadParts any `json:"thread_parts,omitempty"`
}

// TelegramButton is one link button under a Telegram post (one per row).
type TelegramButton struct {
	// Text is the label, 1 to 64 characters.
	Text string `json:"text"`
	// URL is an https://, http:// or tg:// link.
	URL string `json:"url"`
}

// TelegramPoll is a native Telegram poll. The post text is the question
// (max 300 characters); a poll carries no media and no buttons.
type TelegramPoll struct {
	// Options: 2 to 12 answers of up to 100 characters, all different.
	Options []string `json:"options"`
	// OpenPeriod is the number of seconds the poll stays open (5 to
	// 2628000). 0 keeps it open until it is closed in Telegram.
	OpenPeriod int `json:"open_period,omitempty"`
	// Multiple allows several answers per person.
	Multiple *bool `json:"multiple,omitempty"`
}

// TelegramPostOptions holds the Telegram options of a post: the channel or
// group the workspace connected, posted by the OmniSocials bot. Text up to
// 4,096 characters; with media, text over 1,024 characters is sent as a
// reply under the media. Up to 10 photos and videos per album (a GIF goes on
// its own), files up to 50 MB, no Stories.
type TelegramPostOptions struct {
	// Silent sends the post without a notification sound.
	Silent *bool `json:"silent,omitempty"`
	// LinkPreview false hides the link preview of a text-only post.
	LinkPreview *bool `json:"link_preview,omitempty"`
	// Pin pins the post in the chat. It needs the bot's Edit messages right
	// in a channel (Pin messages in a group); without it the post is sent
	// unpinned.
	Pin *bool `json:"pin,omitempty"`
	// ProtectContent stops forwarding and saving in Telegram apps.
	ProtectContent *bool `json:"protect_content,omitempty"`
	// Spoiler blurs the photos and videos until someone taps them.
	Spoiler *bool `json:"spoiler,omitempty"`
	// CaptionAboveMedia shows the text above the media.
	CaptionAboveMedia *bool `json:"caption_above_media,omitempty"`
	// Buttons: up to 3 link buttons. Not on an album (2+ media) or a poll.
	Buttons []TelegramButton `json:"buttons,omitempty"`
	// Poll is a native Telegram poll; see TelegramPoll.
	Poll *TelegramPoll `json:"poll,omitempty"`
	// FirstComment is a reply the bot sends under the post right after it
	// is published (max 4,096 characters).
	FirstComment string `json:"first_comment,omitempty"`
}

// TelegramPostOptionsUpdate is the update-side variant of
// TelegramPostOptions. Every set field merges into the stored options; a
// nil field is left untouched.
type TelegramPostOptionsUpdate struct {
	Silent            *bool `json:"silent,omitempty"`
	LinkPreview       *bool `json:"link_preview,omitempty"`
	Pin               *bool `json:"pin,omitempty"`
	ProtectContent    *bool `json:"protect_content,omitempty"`
	Spoiler           *bool `json:"spoiler,omitempty"`
	CaptionAboveMedia *bool `json:"caption_above_media,omitempty"`
	// Buttons: a []TelegramButton replaces them, omnisocials.Null removes them.
	Buttons any `json:"buttons,omitempty"`
	// Poll: a *TelegramPoll replaces it, omnisocials.Null removes it.
	Poll any `json:"poll,omitempty"`
	// FirstComment: a pointer to an empty string clears the first comment.
	FirstComment *string `json:"first_comment,omitempty"`
}

// DiscordButton is one link button under a Discord post (all in one row).
type DiscordButton struct {
	// Text is the label, 1 to 80 characters.
	Text string `json:"text"`
	// URL is an https:// or http:// link.
	URL string `json:"url"`
}

// DiscordPoll is a native Discord poll. The post text is the question (max
// 300 characters); a poll carries no media and no buttons.
type DiscordPoll struct {
	// Options: 2 to 10 answers of up to 55 characters, all different.
	Options []string `json:"options"`
	// DurationHours is the number of hours the poll stays open (1 to 768).
	// 0 sends none, so Discord's default of 24 hours applies.
	DurationHours int `json:"duration_hours,omitempty"`
	// Multiple allows several answers per person.
	Multiple *bool `json:"multiple,omitempty"`
}

// DiscordPostOptions holds the Discord options of a post: one Discord
// channel, posted through its incoming webhook (the message shows the
// webhook's name and picture). Text up to 2,000 characters. Up to 10
// images, GIFs and videos per message, mixed; a video or GIF over Discord's
// 20 MB attachment limit is sent as a link in the text (an image over it is
// re-encoded). No Stories; a Reel is one video message.
type DiscordPostOptions struct {
	// Silent sends the post without a push or desktop notification
	// (Discord's @silent).
	Silent *bool `json:"silent,omitempty"`
	// LinkPreview false hides the previews (embeds) of links in the text.
	LinkPreview *bool `json:"link_preview,omitempty"`
	// AllowMentions true lets @everyone, @here, role and user mentions in the
	// text ping. Off by default: mentions show as text and ping nobody.
	AllowMentions *bool `json:"allow_mentions,omitempty"`
	// Spoiler blurs every image and video until someone clicks it.
	Spoiler *bool `json:"spoiler,omitempty"`
	// ThreadName is the post title in a forum or media channel, where
	// Discord requires one (1 to 100 characters). On a text channel the post
	// goes out without it.
	ThreadName string `json:"thread_name,omitempty"`
	// Buttons: up to 5 link buttons in one row. Not with a poll.
	Buttons []DiscordButton `json:"buttons,omitempty"`
	// Poll is a native Discord poll; see DiscordPoll.
	Poll *DiscordPoll `json:"poll,omitempty"`
}

// DiscordPostOptionsUpdate is the update-side variant of
// DiscordPostOptions. Every set field merges into the stored options; a nil
// field is left untouched.
type DiscordPostOptionsUpdate struct {
	Silent        *bool `json:"silent,omitempty"`
	LinkPreview   *bool `json:"link_preview,omitempty"`
	AllowMentions *bool `json:"allow_mentions,omitempty"`
	Spoiler       *bool `json:"spoiler,omitempty"`
	// ThreadName: a string replaces it, omnisocials.Null removes it.
	ThreadName any `json:"thread_name,omitempty"`
	// Buttons: a []DiscordButton replaces them, omnisocials.Null removes them.
	Buttons any `json:"buttons,omitempty"`
	// Poll: a *DiscordPoll replaces it, omnisocials.Null removes it.
	Poll any `json:"poll,omitempty"`
}

// ThreadsLocationInput is the object form of a Threads location tag on a
// post: the id plus optional display fields to store along with it. See
// ThreadsPostOptions.Location.
type ThreadsLocationInput struct {
	// ID is a Threads location id from Locations.SearchThreads.
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
}

// ThreadsPostOptions holds Threads (Meta) specific options for post creation.
type ThreadsPostOptions struct {
	// Poll on the post (a text-only post); see ChannelPoll.
	Poll *ChannelPoll `json:"poll,omitempty"`
	// ThreadParts: provide 2-25 parts to publish as a chained thread; parts
	// after the first publish as replies to the previous part, and the
	// Threads caption is taken from part 1. Omit for a single post.
	ThreadParts []ThreadPart `json:"thread_parts,omitempty"`
	// LocationID is a Threads location tag: a Threads location id from
	// Locations.SearchThreads (Facebook Place ids are not interchangeable).
	// On a multi-post thread the tag is applied to part 1. Threads location
	// tagging is currently rolling out; until Meta approves the permissions
	// it is disabled on production and calls return a clear error.
	LocationID string `json:"location_id,omitempty"`
	// Location is an alternative to LocationID: store display fields along
	// with the id. LocationID wins when both are set.
	Location *ThreadsLocationInput `json:"location,omitempty"`
	// TopicTag is the topic of the post on Threads (one per post), e.g.
	// "Build in public". Unlike a #tag in the text it can hold spaces. 1 to
	// 50 characters, no period and no ampersand; a leading "#" is dropped.
	// On a multi-post thread the topic goes on part 1.
	TopicTag string `json:"topic_tag,omitempty"`
}

// ThreadsPostOptionsUpdate is the update-side variant of ThreadsPostOptions.
type ThreadsPostOptionsUpdate struct {
	// Poll: leave nil to keep the existing poll untouched, set a
	// *ChannelPoll to replace it, or set omnisocials.Null to remove it.
	Poll any `json:"poll,omitempty"`
	// ThreadParts: leave nil to keep the existing thread untouched, set a
	// []ThreadPart to replace it, or set omnisocials.Null to clear thread
	// mode (revert to a single post).
	ThreadParts any `json:"thread_parts,omitempty"`
	// LocationID: leave nil to keep the existing location tag untouched, set
	// a string to (re)tag, or set omnisocials.Null to clear the tag.
	LocationID any `json:"location_id,omitempty"`
	// Location: leave nil to keep the existing tag untouched, set a
	// *ThreadsLocationInput to (re)tag with display fields, or set
	// omnisocials.Null to clear. LocationID wins when both are set.
	Location any `json:"location,omitempty"`
	// TopicTag: leave nil to keep the existing topic untouched, set a string
	// to replace it, or set omnisocials.Null to remove it.
	TopicTag any `json:"topic_tag,omitempty"`
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

// ---- Automations (comment-to-DM) ---------------------------------------------

// AutomationTrigger is the trigger block of an Automation.
type AutomationTrigger struct {
	// Kinds is one or more of "comment", "dm", "story_reply". A Facebook
	// Page has "comment" and "dm" only ("dm" = a keyword in a Messenger
	// message to the Page); it has no story replies. An X automation has
	// "dm" only (a keyword in a DM to the X account).
	Kinds []string `json:"kinds"`
	// Scope, for the comment kind: "post" (one post), "next_post" (the next
	// post the workspace publishes on the automation's channel) or "any"
	// (every post). Nil when Kinds has no "comment".
	Scope    *string  `json:"scope,omitempty"`
	Keywords []string `json:"keywords"`
	// Match is "word" (whole word), "contains" (anywhere in the text) or
	// "any" (every message, no keywords needed).
	Match string `json:"match"`
}

// AutomationTriggerInput is the trigger block for Automations.Create and
// Automations.Update.
type AutomationTriggerInput struct {
	// Kinds defaults to ["comment"]. On X pass ["dm"], the only kind there
	// ("comment" or "story_reply" on X is a 400 validation_error).
	Kinds []string `json:"kinds,omitempty"`
	// Scope defaults to "post"; "next_post" = the next post the workspace
	// publishes on the automation's channel, "any" = every post.
	Scope string `json:"scope,omitempty"`
	// Keywords: up to 20, 60 characters each. Required unless Match is "any".
	Keywords []string `json:"keywords,omitempty"`
	// Match defaults to "word".
	Match string `json:"match,omitempty"`
}

// AutomationDM is the DM an automation sends (Text max 1000 characters).
type AutomationDM struct {
	Text string `json:"text"`
}

// AutomationPublicReply is the optional public reply under the comment
// (comment kind only): one of Variations is posted (up to 10 of 300
// characters each).
type AutomationPublicReply struct {
	Enabled    *bool    `json:"enabled,omitempty"`
	Variations []string `json:"variations,omitempty"`
}

// AutomationActions is the actions block of an Automation, also sent on
// create / update and inside CommentDMAutomationInput.
type AutomationActions struct {
	DM          AutomationDM           `json:"dm"`
	PublicReply *AutomationPublicReply `json:"public_reply,omitempty"`
}

// AutomationLimits is the limits block of an Automation, also sent on create
// / update and inside CommentDMAutomationInput. A nil field means no limit.
type AutomationLimits struct {
	// MaxSends stops the automation after this many DMs.
	MaxSends *int `json:"max_sends,omitempty"`
	// StopAt is an ISO 8601 datetime after which the automation stops.
	StopAt *string `json:"stop_at,omitempty"`
}

// AutomationStats are the counters of an Automation.
type AutomationStats struct {
	Matched     int     `json:"matched"`
	Sent        int     `json:"sent"`
	Skipped     int     `json:"skipped"`
	Failed      int     `json:"failed"`
	LastEventAt *string `json:"last_event_at,omitempty"`
}

// AutomationPost is the post an automation listens on, as a snapshot.
type AutomationPost struct {
	// Source is "omnisocials" (a post made here), "instagram" or "facebook"
	// (a post published outside OmniSocials, read from that platform).
	Source       string  `json:"source"`
	ID           string  `json:"id"`
	Caption      *string `json:"caption,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Permalink    *string `json:"permalink,omitempty"`
	// MediaType is "POST", "REEL", "IMAGE", "VIDEO" or "CAROUSEL_ALBUM".
	MediaType *string `json:"media_type,omitempty"`
	// Status, PostDateTime and Live are set for the "omnisocials" source
	// only; Live is true once the post is live on the automation's channel.
	Status       string  `json:"status,omitempty"`
	PostDateTime *string `json:"post_date_time,omitempty"`
	Live         *bool   `json:"live,omitempty"`
}

// Automation is a comment-to-DM automation: a keyword in an Instagram
// comment (or a DM, or a story reply) is answered with a DM, once per
// person, with an optional public reply under the comment. On a Facebook
// Page a keyword in a comment (or a Messenger message to the Page) is
// answered with a DM in Messenger; a Page has no story replies. Meta rules,
// the same on both: one private reply per comment, within 7 days;
// OmniSocials caps sends at 180 DMs an hour per account and platform. On X
// a keyword in a DM to the X account is answered with a DM (no post, no
// public reply); every X answer ends with "Reply STOP to stop automatic
// messages." and a person who replies STOP gets no automatic DM from the
// account again; X caps sends at 15 per 15 minutes and 450 a day. An answer
// over a cap waits in a queue (outcome "queued") for up to an hour.
type Automation struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// Platform is "instagram", "facebook" (a Facebook Page) or "x" (the X
	// account). Set at create; it never changes.
	Platform string `json:"platform"`
	// Status is "active", "paused" or "completed".
	Status string  `json:"status"`
	Name   *string `json:"name,omitempty"`
	// PostID is the OmniSocials post it listens on; nil when none.
	PostID *string `json:"post_id,omitempty"`
	// InstagramMediaID is the Instagram media id it listens on; set once the
	// post is live. Nil for a Facebook automation.
	InstagramMediaID *string `json:"instagram_media_id,omitempty"`
	// FacebookPostID is the Facebook Page post id it listens on
	// ("{page-id}_{post-id}"); set once the post is live. Nil for an
	// Instagram automation.
	FacebookPostID *string `json:"facebook_post_id,omitempty"`
	// Post is a snapshot of that post; nil for scope "next_post" or "any".
	Post      *AutomationPost   `json:"post,omitempty"`
	Trigger   AutomationTrigger `json:"trigger"`
	Actions   AutomationActions `json:"actions"`
	Limits    AutomationLimits  `json:"limits"`
	Stats     AutomationStats   `json:"stats"`
	CreatedAt string            `json:"created_at,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty"`
	// Platforms is set on Post.CommentDMAutomation only: the channels with an
	// automation on that post, e.g. ["instagram", "facebook"]. The other
	// fields are those of the first one (Instagram first).
	Platforms []string `json:"platforms,omitempty"`
	// Automations is set on Post.CommentDMAutomation only: one entry per
	// channel.
	Automations []AutomationSummary `json:"automations,omitempty"`
}

// AutomationSummary is one entry of Automation.Automations: the automation
// of one channel on a post.
type AutomationSummary struct {
	ID string `json:"id"`
	// Platform is "instagram" or "facebook".
	Platform string `json:"platform"`
	// Status is "active", "paused" or "completed".
	Status string `json:"status"`
}

// AutomationEventPerson is the person an AutomationEvent is about. On
// Facebook ID is the person's id as the Page sees it and Username is their
// name (Facebook has no @handle); on Instagram and X Username is the handle.
type AutomationEventPerson struct {
	ID       string  `json:"id"`
	Username *string `json:"username,omitempty"`
}

// AutomationEvent is one entry of Automations.ListEvents: what happened for
// one person.
type AutomationEvent struct {
	ID           string `json:"id"`
	AutomationID string `json:"automation_id"`
	// Kind is what the person did: "comment", "dm" or "story_reply".
	Kind string `json:"kind"`
	// Outcome is "sent", "skipped", "failed", "pending", "queued" (the
	// answer waits for a free slot under the send cap and goes out within
	// the hour) or "opted_out" (the person replied STOP on X).
	Outcome string `json:"outcome"`
	// Reason: for "skipped" one of "already_sent", "limit_reached",
	// "stopped", "window_passed", "rate_limited" (the send cap stayed full
	// for an hour), "cannot_message" (a Page made the comment, or the
	// person's settings block messages), "already_replied",
	// "dm_window_passed", "dms_closed" (X: the person does not accept DMs
	// from the account), "no_credits" (X DMs had no credits for the
	// answer), "opted_out" (the person replied STOP before),
	// "automation_off" (paused or finished while the answer waited); for
	// "failed" the platform's error text.
	Reason *string `json:"reason,omitempty"`
	// ReasonLabel is Reason in plain words.
	ReasonLabel *string                `json:"reason_label,omitempty"`
	Person      *AutomationEventPerson `json:"person,omitempty"`
	// TriggerID is the comment id or message id that triggered the event.
	TriggerID *string `json:"trigger_id,omitempty"`
	// TriggerText is the comment text; for "dm" and "story_reply" only with
	// the inbox:read scope, else nil.
	TriggerText *string `json:"trigger_text,omitempty"`
	// Message is the DM that was sent.
	Message           *string `json:"message,omitempty"`
	PublicReply       *string `json:"public_reply,omitempty"`
	PlatformMessageID *string `json:"platform_message_id,omitempty"`
	CreatedAt         string  `json:"created_at,omitempty"`
	SentAt            *string `json:"sent_at,omitempty"`
}

// AutomationEventsResponse is the Automations.ListEvents response. Note the
// shape is not the plain list envelope: next to Data it carries Counts, the
// number of events per outcome (e.g. {"sent": 12, "skipped": 3}; an outcome
// with no events is left out), and Pagination.
type AutomationEventsResponse struct {
	Data       []AutomationEvent `json:"data"`
	Counts     map[string]int    `json:"counts"`
	Pagination *Pagination       `json:"pagination,omitempty"`
}

// AutomationCreateParams is the request body for Automations.Create.
type AutomationCreateParams struct {
	// Platform is "instagram" (default), "facebook" (a Facebook Page; the
	// DM lands in Messenger) or "x" (the X account: Trigger.Kinds ["dm"]
	// only, no post fields). FacebookPostID implies "facebook". Facebook
	// has no story replies: "story_reply" in Trigger.Kinds is a 400
	// validation_error. An active X automation needs X DMs on for the
	// workspace (409 x_dms_off; Status "paused" always saves).
	Platform string `json:"platform,omitempty"`
	// Name, max 120 characters.
	Name string `json:"name,omitempty"`
	// PostID, for scope "post": an OmniSocials post that goes to the
	// automation's channel (scheduled or live). One automation per post and
	// platform: a post that goes to Instagram and Facebook can have one of
	// each. Pass at most one of PostID, InstagramMediaID and FacebookPostID.
	PostID string `json:"post_id,omitempty"`
	// InstagramMediaID, for scope "post": an Instagram post published
	// outside OmniSocials (numeric Instagram media id).
	InstagramMediaID string `json:"instagram_media_id,omitempty"`
	// FacebookPostID, for scope "post": a post published on the Facebook
	// Page outside OmniSocials, as "{page-id}_{post-id}" or the post id
	// alone (the Page id is put in front). Implies Platform "facebook". A
	// post of another Page is a 404 facebook_post_not_found.
	FacebookPostID string                  `json:"facebook_post_id,omitempty"`
	Trigger        *AutomationTriggerInput `json:"trigger,omitempty"`
	// Actions is required; Actions.DM.Text is the DM to send.
	Actions AutomationActions `json:"actions"`
	Limits  *AutomationLimits `json:"limits,omitempty"`
	// Status is "active" (default) or "paused" (create as paused).
	Status string `json:"status,omitempty"`
}

// AutomationUpdateParams is the request body for Automations.Update. Only
// set fields are sent; a body with nothing is a 400 no_fields.
type AutomationUpdateParams struct {
	// Platform cannot change: another platform (or InstagramMediaID on a
	// Facebook automation, FacebookPostID on an Instagram one) is a 400
	// platform_immutable.
	Platform string `json:"platform,omitempty"`
	Name     string `json:"name,omitempty"`
	// PostID / InstagramMediaID / FacebookPostID move the automation to
	// that post.
	PostID           string `json:"post_id,omitempty"`
	InstagramMediaID string `json:"instagram_media_id,omitempty"`
	FacebookPostID   string `json:"facebook_post_id,omitempty"`
	// Trigger is merged into the stored trigger: fields you leave out keep
	// their value, so new Keywords keep the kinds, the scope and the match.
	Trigger *AutomationTriggerInput `json:"trigger,omitempty"`
	// Actions is merged into the stored actions: a new DM.Text keeps the
	// public reply. PublicReply with Enabled omnisocials.Bool(false) turns
	// the public reply off.
	Actions *AutomationActions `json:"actions,omitempty"`
	// Limits is merged into the stored limits: a *AutomationLimits with only
	// StopAt keeps MaxSends. To remove a limit send a map[string]any with
	// omnisocials.Null, e.g. map[string]any{"max_sends": omnisocials.Null}
	// (a nil pointer field is omitted, so it cannot express the null).
	Limits any `json:"limits,omitempty"`
	// Status "paused" pauses, "active" resumes (an X automation only while
	// X DMs are on for the workspace, else 409 x_dms_off).
	Status string `json:"status,omitempty"`
}

// AutomationListParams filters Automations.List.
type AutomationListParams struct {
	// Status is "active", "paused" or "completed".
	Status string
	// Platform is "instagram", "facebook" or "x": only that channel's
	// automations.
	Platform string
	// PostID returns only the automation bound to this OmniSocials post.
	PostID string
}

// AutomationEventListParams filters Automations.ListEvents.
type AutomationEventListParams struct {
	// Outcome is "sent", "skipped", "failed", "pending", "queued" or
	// "opted_out".
	Outcome string
	// Limit is 1 to 100 (default 25).
	Limit int
	// Offset is the number of events to skip.
	Offset int
}

// CommentDMAutomationTrigger is the trigger of a CommentDMAutomationInput:
// the keywords and how they match (the post itself is the scope).
type CommentDMAutomationTrigger struct {
	// Keywords: up to 20, 60 characters each. Required unless Match is "any".
	Keywords []string `json:"keywords,omitempty"`
	// Match is "word" (default), "contains" or "any".
	Match string `json:"match,omitempty"`
}

// CommentDMAutomationInput is PostCreateParams.CommentDMAutomation and
// PostUpdateParams.CommentDMAutomation: the comment-to-DM automation bound to
// that post (the Automations.Create body without post_id,
// instagram_media_id, facebook_post_id, platform, scope, kinds, name and
// status, plus Platforms; Instagram or Facebook must be one of the post's
// channels). Needs the automations:write scope on the key.
type CommentDMAutomationInput struct {
	Trigger *CommentDMAutomationTrigger `json:"trigger,omitempty"`
	Actions AutomationActions           `json:"actions"`
	Limits  *AutomationLimits           `json:"limits,omitempty"`
	// Platforms are the channels that get the automation ("instagram",
	// "facebook"): one automation per listed channel, each with the same
	// keywords, DM, public reply and limits. Each one must be a channel of
	// the post (else 400 validation_error). Leave it nil for Instagram when
	// the post goes there, else Facebook. On update a channel left out loses
	// its automation.
	Platforms []string `json:"platforms,omitempty"`
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
