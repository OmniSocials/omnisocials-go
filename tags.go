package omnisocials

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// TagsService covers the /tags endpoints: post tags, the workspace's own
// labels for its posts (a hook style, a format) to filter and compare posts
// in Analytics. Tags never go to a platform. Set them on a post with
// PostCreateParams.Tags / PostUpdateParams.Tags, filter with
// PostListParams.Tag and AnalyticsOverviewParams.Tag, change many posts at
// once with Tags.Apply, and compare them with Analytics.Tags. Uses the
// posts:read / posts:write scopes.
//
// Error codes: 404 tag_not_found; 409 tag_exists (the existing tag is in the
// error body under error.tag), tag_limit_reached (200 tags per
// workspace); 400 invalid_tag_name (1 to 40 characters), invalid_tag_color,
// invalid_tags, invalid_post_ids, too_many_posts (500 per Apply),
// too_many_tags (20 per post).
type TagsService struct {
	client *Client
}

// TagColors are the colours a tag can have.
var TagColors = []string{"blue", "green", "orange", "red", "purple", "pink", "teal", "yellow", "brown", "gray"}

// Tag is a workspace's own label for its posts.
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Color is one of TagColors.
	Color     string `json:"color"`
	CreatedAt string `json:"created_at,omitempty"`
}

// TrialReel is set on a post that went to Instagram as a Trial Reel.
// Strategy is "SS_PERFORMANCE" (Instagram shares it with followers when it
// does well) or "MANUAL" (the person decides in the Instagram app).
// Instagram's API does not say whether a trial reel was later shared.
type TrialReel struct {
	Strategy string `json:"strategy"`
}

// TagCreateParams is the request body for Tags.Create.
type TagCreateParams struct {
	// Name is unique in the workspace (case-insensitive), 1 to 40
	// characters.
	Name string `json:"name"`
	// Color is one of TagColors. Omit to get the next colour of the palette.
	Color string `json:"color,omitempty"`
}

// TagUpdateParams is the request body for Tags.Update. A field you leave
// empty keeps its value; posts keep the tag.
type TagUpdateParams struct {
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

// TagApplyParams is the request body for Tags.Apply. Add, Remove and Set take
// tag ids (int, int64) or names (string), mixed as you like; a name matches
// case-insensitively and a new name in Add or Set becomes a tag.
type TagApplyParams struct {
	// PostIDs are the posts to change, at most 500. Ids that are not posts
	// of this workspace come back in TagApplyResult.NotFound.
	PostIDs []string `json:"post_ids"`
	// Add adds these tags and keeps the others.
	Add []any `json:"add,omitempty"`
	// Remove takes these tags off.
	Remove []any `json:"remove,omitempty"`
	// Set replaces each post's tags: a []any of ids or names. An empty
	// []any{} removes every tag. Leave nil to send no "set".
	Set any `json:"set,omitempty"`
}

// TagApplyPost is one post of a TagApplyResult with its tags now.
type TagApplyPost struct {
	ID   string `json:"id"`
	Tags []Tag  `json:"tags"`
}

// TagApplyResult is the data payload of Tags.Apply.
type TagApplyResult struct {
	// Updated is the number of posts whose tags changed.
	Updated  int            `json:"updated"`
	Posts    []TagApplyPost `json:"posts"`
	NotFound []string       `json:"not_found"`
}

// TagDeleteResult is the data payload of Tags.Delete.
type TagDeleteResult struct {
	Tag Tag `json:"tag"`
	// PostsUntagged is the number of posts that carried the tag.
	PostsUntagged int `json:"posts_untagged"`
}

// TagSummary is how a set of posts did against the workspace's normal.
type TagSummary struct {
	Posts int `json:"posts"`
	// Measured is the number of posts with numbers.
	Measured          int      `json:"measured"`
	MedianImpressions *float64 `json:"median_impressions"`
	// EngagementRate is a percentage (5.4 = 5.4%).
	EngagementRate *float64 `json:"engagement_rate"`
	// VsNormal is a fraction against the normal: 0.38 = 38% above, -0.2 =
	// 20% below. Each post is measured against the median impressions of
	// its own channel in the period.
	VsNormal *float64 `json:"vs_normal"`
	// BestChannel is the channel where the tag beats the normal by the most
	// (3 posts there at least).
	BestChannel *string `json:"best_channel"`
	// EnoughPosts is false below MinPosts measured posts: read VsNormal as a
	// hint only then.
	EnoughPosts bool `json:"enough_posts"`
}

// TagComparison is one tag's TagSummary.
type TagComparison struct {
	TagSummary
	Tag Tag `json:"tag"`
}

// TagAnalyticsPeriod is the period of a TagAnalytics.
type TagAnalyticsPeriod struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// TagAnalytics is the data payload of Analytics.Tags.
type TagAnalytics struct {
	Period   TagAnalyticsPeriod `json:"period"`
	Platform *string            `json:"platform"`
	MinPosts int                `json:"min_posts"`
	// Tags come comparable first, best first.
	Tags []TagComparison `json:"tags"`
	// Untagged is the posts without a tag.
	Untagged TagSummary `json:"untagged"`
	// Normals is the median impressions per channel in the period.
	Normals map[string]*float64 `json:"normals"`
}

// TagAnalyticsParams filters Analytics.Tags.
type TagAnalyticsParams struct {
	// StartDate is "YYYY-MM-DD", "YYYY-MM" or ISO 8601. Default: 90 days ago.
	StartDate string
	// EndDate is "YYYY-MM-DD", "YYYY-MM" or ISO 8601 (inclusive). Default:
	// today.
	EndDate string
	// Platform keeps one channel's numbers ("instagram", "tiktok", ...).
	// Default: every connected channel.
	Platform string
	// MinPosts is the number of measured posts a tag needs before
	// EnoughPosts is true (1 to 50, default 5).
	MinPosts int
}

func tagPath(id int64) string {
	return "/tags/" + url.PathEscape(strconv.FormatInt(id, 10))
}

// List calls `GET /tags`: every tag of the workspace, by name.
func (s *TagsService) List(ctx context.Context) (*ListResponse[Tag], error) {
	var out ListResponse[Tag]
	if err := s.client.get(ctx, "/tags", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /tags`: create a tag (at most 200 per workspace). A
// name that exists answers 409 tag_exists with the existing tag in the error
// body.
func (s *TagsService) Create(ctx context.Context, params *TagCreateParams) (*ItemResponse[Tag], error) {
	var out ItemResponse[Tag]
	if err := s.client.post(ctx, "/tags", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /tags/:id`: rename or recolour a tag.
func (s *TagsService) Update(ctx context.Context, id int64, params *TagUpdateParams) (*ItemResponse[Tag], error) {
	var out ItemResponse[Tag]
	if err := s.client.patch(ctx, tagPath(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /tags/:id`: delete the tag and take it off every post
// of the workspace. The posts themselves stay.
func (s *TagsService) Delete(ctx context.Context, id int64) (*ItemResponse[TagDeleteResult], error) {
	var out ItemResponse[TagDeleteResult]
	if err := s.client.do(ctx, http.MethodDelete, tagPath(id), nil, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Apply calls `POST /tags/apply`: add, remove or replace the tags of up to
// 500 posts at once, published ones included.
func (s *TagsService) Apply(ctx context.Context, params *TagApplyParams) (*ItemResponse[TagApplyResult], error) {
	var out ItemResponse[TagApplyResult]
	if err := s.client.post(ctx, "/tags/apply", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Tags calls `GET /analytics/tags`: how each tag did against the
// workspace's normal for the published posts in a period (the Tags tab in
// Analytics). Requires the analytics:read scope.
func (s *AnalyticsService) Tags(ctx context.Context, params *TagAnalyticsParams) (*ItemResponse[TagAnalytics], error) {
	query := url.Values{}
	if params != nil {
		if params.StartDate != "" {
			query.Set("start_date", params.StartDate)
		}
		if params.EndDate != "" {
			query.Set("end_date", params.EndDate)
		}
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.MinPosts > 0 {
			query.Set("min_posts", strconv.Itoa(params.MinPosts))
		}
	}
	var out ItemResponse[TagAnalytics]
	if err := s.client.get(ctx, "/analytics/tags", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
