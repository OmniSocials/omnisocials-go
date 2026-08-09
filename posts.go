package omnisocials

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// PostsService covers the /posts endpoints.
type PostsService struct {
	client *Client
}

// PostCreateParams is the request body for Posts.Create and
// Posts.CreateAndPublish (ScheduledAt is ignored by the latter).
type PostCreateParams struct {
	// Content is the caption: either a plain string, or a
	// map[string]string of per-platform captions with a "default" key.
	Content any `json:"content"`
	// Channels are platform identifiers, e.g. ["instagram", "linkedin", "x"].
	Channels []string `json:"channels,omitempty"`
	// ScheduledAt is an ISO 8601 datetime. Omit to create a draft.
	ScheduledAt string `json:"scheduled_at,omitempty"`
	// MediaIDs are Media Library ids: either a []string shared across
	// platforms, or a map[string][]string keyed by platform. Entries may
	// also be MediaIDEntry values ({id, alt}) for per-media alt text.
	MediaIDs any `json:"media_ids,omitempty"`
	// MediaURLs are public media URLs: either a []string shared across
	// platforms, or a map[string][]string keyed by platform. Entries may
	// also be MediaURLEntry values ({url, alt}) for per-media alt text.
	MediaURLs any `json:"media_urls,omitempty"`
	// Type is the post type, e.g. "post", "story", "reel".
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
	// Link share fields (LinkedIn / Facebook).
	LinkURL          string `json:"link_url,omitempty"`
	LinkTitle        string `json:"link_title,omitempty"`
	LinkDescription  string `json:"link_description,omitempty"`
	LinkThumbnailURL string `json:"link_thumbnail_url,omitempty"`
	// LocationID is an Instagram location tag (a Facebook Place id, see
	// Locations.Search).
	LocationID string `json:"location_id,omitempty"`
	// Collaborators are Instagram co-author usernames.
	Collaborators []string  `json:"collaborators,omitempty"`
	UserTags      []UserTag `json:"user_tags,omitempty"`
	// HashtagSet applies a saved hashtag set by name (case-insensitive) once
	// at create time; tags already in a caption are skipped; Instagram's
	// 30-hashtag cap returns error code hashtag_limit_exceeded.
	HashtagSet string `json:"hashtag_set,omitempty"`
	// HashtagSetID applies a saved hashtag set by id (see HashtagSet).
	HashtagSetID string `json:"hashtag_set_id,omitempty"`
	// HashtagPlacement is where the hashtags go: "caption_append" (default)
	// or "first_comment".
	HashtagPlacement string `json:"hashtag_placement,omitempty"`
	// HashtagPlatforms restricts the hashtags to a subset of Channels.
	HashtagPlatforms []string `json:"hashtag_platforms,omitempty"`
	// Per-platform options.
	Pinterest      map[string]any       `json:"pinterest,omitempty"`
	YouTube        map[string]any       `json:"youtube,omitempty"`
	Instagram      map[string]any       `json:"instagram,omitempty"`
	Facebook       map[string]any       `json:"facebook,omitempty"`
	LinkedIn       map[string]any       `json:"linkedin,omitempty"`
	LinkedInPage   map[string]any       `json:"linkedin_page,omitempty"`
	TikTok         map[string]any       `json:"tiktok,omitempty"`
	X              *XPostOptions        `json:"x,omitempty"`
	Bluesky        *BlueskyPostOptions  `json:"bluesky,omitempty"`
	Mastodon       *MastodonPostOptions `json:"mastodon,omitempty"`
	GoogleBusiness map[string]any       `json:"google_business,omitempty"`
}

// PostUpdateParams is the request body for Posts.Update. Only set fields are
// sent; drafts and scheduled posts can be updated.
type PostUpdateParams struct {
	// Content: plain string or map[string]string per-platform captions.
	Content     any      `json:"content,omitempty"`
	ScheduledAt string   `json:"scheduled_at,omitempty"`
	Channels    []string `json:"channels,omitempty"`
	// MediaIDs / MediaURLs: same shapes as PostCreateParams, including
	// MediaIDEntry / MediaURLEntry values for per-media alt text.
	MediaIDs      any            `json:"media_ids,omitempty"`
	MediaURLs     any            `json:"media_urls,omitempty"`
	Type          string         `json:"type,omitempty"`
	LocationID    string         `json:"location_id,omitempty"`
	Collaborators []string       `json:"collaborators,omitempty"`
	UserTags      []UserTag      `json:"user_tags,omitempty"`
	Pinterest     map[string]any `json:"pinterest,omitempty"`
	YouTube       map[string]any `json:"youtube,omitempty"`
	Instagram     map[string]any `json:"instagram,omitempty"`
	Facebook      map[string]any `json:"facebook,omitempty"`
	LinkedIn      map[string]any `json:"linkedin,omitempty"`
	LinkedInPage  map[string]any `json:"linkedin_page,omitempty"`
	TikTok        map[string]any `json:"tiktok,omitempty"`
	// X: set ThreadParts to omnisocials.Null to clear thread mode; omit to
	// leave the existing thread untouched.
	X *XPostOptionsUpdate `json:"x,omitempty"`
	// Bluesky: set ThreadParts to omnisocials.Null to clear thread mode.
	Bluesky *BlueskyPostOptionsUpdate `json:"bluesky,omitempty"`
	// Mastodon: set ThreadParts to omnisocials.Null to clear thread mode.
	Mastodon       *MastodonPostOptionsUpdate `json:"mastodon,omitempty"`
	GoogleBusiness map[string]any             `json:"google_business,omitempty"`
}

// PostListParams filters Posts.List.
type PostListParams struct {
	// Status filters by post status, e.g. "draft", "scheduled", "published",
	// "failed".
	Status string
	// Limit is the max items to return (default 20, max 100).
	Limit int
	// Offset is the number of items to skip (default 0).
	Offset int
}

// RecentPlatformPostsParams filters Posts.RecentPlatform.
type RecentPlatformPostsParams struct {
	// Limit is the max posts per platform.
	Limit int
	// Platforms restricts the fetch, e.g. ["instagram", "x"]. Empty fetches
	// all connected platforms.
	Platforms []string
}

// Post is a post as returned by the API.
type Post struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Type   string `json:"type,omitempty"`
	// Content is either a plain string or a per-platform map (decoded as
	// map[string]any) with a "default" key.
	Content any `json:"content"`
	// Accounts are the platform identifiers the post targets.
	Accounts []string `json:"accounts,omitempty"`
	// Media is either a []string of URLs shared across platforms or a
	// per-platform map. Items include an optional "alt" accessibility
	// description when one was set.
	Media any `json:"media,omitempty"`
	// ScheduleAt is the scheduled publish time (nil for drafts).
	ScheduleAt     *string `json:"schedule_at,omitempty"`
	ApprovalStatus *string `json:"approval_status,omitempty"`
	// AppURL is a deep link to this post inside the OmniSocials app.
	AppURL string `json:"app_url,omitempty"`
	// PublishedURLs maps platform -> live post URL once published.
	PublishedURLs    map[string]string `json:"published_urls,omitempty"`
	LocationID       string            `json:"location_id,omitempty"`
	Collaborators    []string          `json:"collaborators,omitempty"`
	UserTags         []UserTag         `json:"user_tags,omitempty"`
	LinkURL          string            `json:"link_url,omitempty"`
	LinkTitle        string            `json:"link_title,omitempty"`
	LinkDescription  string            `json:"link_description,omitempty"`
	LinkThumbnailURL string            `json:"link_thumbnail_url,omitempty"`
	// Per-platform options echoed back in the request shape (X, Bluesky, and
	// Mastodon include thread_parts; comment-capable platforms include
	// first_comment / first_comment_result).
	Pinterest      map[string]any `json:"pinterest,omitempty"`
	YouTube        map[string]any `json:"youtube,omitempty"`
	Instagram      map[string]any `json:"instagram,omitempty"`
	Facebook       map[string]any `json:"facebook,omitempty"`
	LinkedIn       map[string]any `json:"linkedin,omitempty"`
	LinkedInPage   map[string]any `json:"linkedin_page,omitempty"`
	TikTok         map[string]any `json:"tiktok,omitempty"`
	X              map[string]any `json:"x,omitempty"`
	Bluesky        map[string]any `json:"bluesky,omitempty"`
	Mastodon       map[string]any `json:"mastodon,omitempty"`
	GoogleBusiness map[string]any `json:"google_business,omitempty"`
	// Errors maps platform -> user-friendly error message. Populated when
	// Status is "failed" or "warning"; only failed platforms appear.
	Errors    map[string]string `json:"errors,omitempty"`
	Source    *string           `json:"source,omitempty"`
	CreatedAt string            `json:"created_at,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty"`
	// RetryOf is set when this post was created by the dashboard "retry as
	// new post" flow: the ID of the original failed post it retries.
	RetryOf string `json:"retry_of,omitempty"`
	// Retries is set on a post that has been retried as a new post: the IDs
	// of its retry posts. A "published" post with empty PublishedURLs and
	// Retries set is a resolved failure, not a second publish.
	Retries []string `json:"retries,omitempty"`
}

// PostRetryResult is the data payload of Posts.Retry.
type PostRetryResult struct {
	// ID of the post being retried.
	ID string `json:"id"`
	// Status of the post after queueing, e.g. "posting".
	Status string `json:"status"`
	// Platforms are the failed platforms being retried.
	Platforms []string `json:"platforms"`
	// Message is a human-readable confirmation.
	Message string `json:"message"`
}

// RecentPlatformPostsResponse is the Posts.RecentPlatform response: recent
// posts fetched live from the connected platform APIs.
type RecentPlatformPostsResponse struct {
	Data  []map[string]any `json:"data"`
	Count int              `json:"count"`
	// ConnectedPlatforms lists the workspace's connected platforms.
	ConnectedPlatforms []string `json:"connected_platforms,omitempty"`
	// Errors maps platform -> fetch error, for platforms that failed.
	Errors      map[string]any `json:"errors,omitempty"`
	Note        string         `json:"note,omitempty"`
	CurrentDate string         `json:"current_date,omitempty"`
}

// List calls `GET /posts`: posts in the workspace, newest first.
func (s *PostsService) List(ctx context.Context, params *PostListParams) (*ListResponse[Post], error) {
	query := url.Values{}
	if params != nil {
		if params.Status != "" {
			query.Set("status", params.Status)
		}
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset > 0 {
			query.Set("offset", strconv.Itoa(params.Offset))
		}
	}
	var out ListResponse[Post]
	if err := s.client.get(ctx, "/posts", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /posts/:id`: fetch a single post.
func (s *PostsService) Get(ctx context.Context, id string) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.get(ctx, "/posts/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecentPlatform calls `GET /posts/recent-platform`: recent posts fetched
// live from the connected platform APIs (including content published outside
// OmniSocials). The fallback for brand-new workspaces where List is empty.
// Requires the analytics:read scope.
func (s *PostsService) RecentPlatform(ctx context.Context, params *RecentPlatformPostsParams) (*RecentPlatformPostsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if len(params.Platforms) > 0 {
			query.Set("platforms", strings.Join(params.Platforms, ","))
		}
	}
	var out RecentPlatformPostsResponse
	if err := s.client.get(ctx, "/posts/recent-platform", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /posts/create`: create a draft or scheduled post. When
// the post targets X and its text (or any thread part) contains a URL, the
// response's Warnings field carries a PostWarning with code
// "x_url_post_credits" (X's link-post fee, passed through as prepaid credits
// at publish time).
//
// Separately, a non-draft Create (ScheduledAt set) reserves the post's
// credit cost up front: if that would push the company's total reserved
// credits for scheduled X link posts past its balance, Create returns a 402
// *APIError with Code "x_credits_insufficient" and Body carrying
// error.details {credits_required, credits_balance, credits_reserved} (402
// isn't one of the typed subclasses, so match it with errors.As against
// *APIError and check Status/Code). Drafts are never gated, and posts
// publishing before 2026-08-14 are never gated. The same gate applies to
// CreateAndPublish, Update, and Publish. It is distinct from the
// x_url_post_credits warning above (create-time, non-blocking) and from the
// publish-time-only-X-target-fails behavior once enforcement starts.
func (s *PostsService) Create(ctx context.Context, params *PostCreateParams) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/create", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateAndPublish calls `POST /posts/create-and-publish`: create a post and
// publish it immediately. See Create for the Warnings field on X link posts
// and the 402 "x_credits_insufficient" schedule-time credit gate (it also
// applies here, since this is a non-draft create).
func (s *PostsService) CreateAndPublish(ctx context.Context, params *PostCreateParams) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/create-and-publish", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /posts/:id`: update a draft or scheduled post. See
// Create for the 402 "x_credits_insufficient" schedule-time credit gate: it
// also applies here when the update schedules a draft, adds X, or edits the
// text/link of a scheduled X link post.
func (s *PostsService) Update(ctx context.Context, id string, params *PostUpdateParams) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.patch(ctx, "/posts/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /posts/:id`: delete a post (204 on success).
func (s *PostsService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/posts/"+url.PathEscape(id))
}

// Publish calls `POST /posts/:id/publish`: publish a draft or scheduled post
// now. See Create for the 402 "x_credits_insufficient" schedule-time credit
// gate: it also runs here as a pre-flight check before an X link post is
// queued to publish.
func (s *PostsService) Publish(ctx context.Context, id string) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/"+url.PathEscape(id)+"/publish", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retry calls `POST /posts/:id/retry`: retry the failed platforms of a
// "failed" or "warning" (partially failed) post, on the same post. Only the
// platforms that failed are re-published; platforms that already succeeded
// are never posted again. Asynchronous: a 200 means the retry is queued -
// poll Get for the outcome. Max 3 retries per platform.
func (s *PostsService) Retry(ctx context.Context, id string) (*ItemResponse[PostRetryResult], error) {
	var out ItemResponse[PostRetryResult]
	if err := s.client.post(ctx, "/posts/"+url.PathEscape(id)+"/retry", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
