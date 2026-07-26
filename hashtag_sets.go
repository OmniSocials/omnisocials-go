package omnisocials

import (
	"context"
	"net/url"
)

// HashtagSetsService covers the /hashtag-sets endpoints (saved, reusable
// hashtag groups). Apply a set to a post at create time via
// PostCreateParams.HashtagSet (name) or PostCreateParams.HashtagSetID.
type HashtagSetsService struct {
	client *Client
}

// HashtagSet is a saved, reusable group of hashtags.
type HashtagSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Hashtags are the tags WITHOUT the leading '#'.
	Hashtags     []string `json:"hashtags"`
	HashtagCount int      `json:"hashtag_count,omitempty"`
	// Preview is the ready-to-paste form, e.g. "#a #b".
	Preview   string `json:"preview,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// HashtagSetCreateParams is the request body for HashtagSets.Create.
type HashtagSetCreateParams struct {
	// Name of the set; posts match it case-insensitively via
	// PostCreateParams.HashtagSet.
	Name string `json:"name"`
	// Hashtags: a []string of tags, or a single string of tags.
	Hashtags any `json:"hashtags"`
}

// HashtagSetUpdateParams is the request body for HashtagSets.Update.
type HashtagSetUpdateParams struct {
	// Name renames the set.
	Name string `json:"name,omitempty"`
	// Hashtags replaces the FULL list: a []string of tags, or a single
	// string of tags.
	Hashtags any `json:"hashtags,omitempty"`
}

// List calls `GET /hashtag-sets`: the workspace's saved hashtag sets.
func (s *HashtagSetsService) List(ctx context.Context) (*ListResponse[HashtagSet], error) {
	var out ListResponse[HashtagSet]
	if err := s.client.get(ctx, "/hashtag-sets", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /hashtag-sets/:id`: fetch a single hashtag set.
func (s *HashtagSetsService) Get(ctx context.Context, id string) (*ItemResponse[HashtagSet], error) {
	var out ItemResponse[HashtagSet]
	if err := s.client.get(ctx, "/hashtag-sets/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /hashtag-sets`: create a hashtag set. Hashtags is a
// []string of tags, or a single string of tags.
func (s *HashtagSetsService) Create(ctx context.Context, params *HashtagSetCreateParams) (*ItemResponse[HashtagSet], error) {
	var out ItemResponse[HashtagSet]
	if err := s.client.post(ctx, "/hashtag-sets", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /hashtag-sets/:id`: rename (Name) and/or replace the
// tags (Hashtags replaces the FULL list).
func (s *HashtagSetsService) Update(ctx context.Context, id string, params *HashtagSetUpdateParams) (*ItemResponse[HashtagSet], error) {
	var out ItemResponse[HashtagSet]
	if err := s.client.patch(ctx, "/hashtag-sets/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /hashtag-sets/:id`: delete a hashtag set (204 on
// success).
func (s *HashtagSetsService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/hashtag-sets/"+url.PathEscape(id))
}
