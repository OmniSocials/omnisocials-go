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
	// platforms, or a map[string][]string keyed by platform.
	MediaIDs any `json:"media_ids,omitempty"`
	// MediaURLs are public media URLs: either a []string shared across
	// platforms, or a map[string][]string keyed by platform.
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
	Content       any            `json:"content,omitempty"`
	ScheduledAt   string         `json:"scheduled_at,omitempty"`
	Channels      []string       `json:"channels,omitempty"`
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
	// per-platform map.
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

// Create calls `POST /posts/create`: create a draft or scheduled post.
func (s *PostsService) Create(ctx context.Context, params *PostCreateParams) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/create", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateAndPublish calls `POST /posts/create-and-publish`: create a post and
// publish it immediately.
func (s *PostsService) CreateAndPublish(ctx context.Context, params *PostCreateParams) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/create-and-publish", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /posts/:id`: update a draft or scheduled post.
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
// now.
func (s *PostsService) Publish(ctx context.Context, id string) (*ItemResponse[Post], error) {
	var out ItemResponse[Post]
	if err := s.client.post(ctx, "/posts/"+url.PathEscape(id)+"/publish", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
