package omnisocials

import (
	"context"
	"net/url"
)

// LocationsService covers the /locations endpoints (Instagram place tagging).
type LocationsService struct {
	client *Client
}

// LocationSearchItem is one Facebook Place from Locations.Search.
type LocationSearchItem struct {
	// ID is the Facebook Place id; use it as a post's LocationID.
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Address *string `json:"address,omitempty"`
	City    *string `json:"city,omitempty"`
	Country *string `json:"country,omitempty"`
}

// LocationSearchResponse is the Locations.Search response. Note: unlike the
// usual envelope, Error here is a plain string set when search is unavailable
// (e.g. no Facebook account connected), with Data then empty.
type LocationSearchResponse struct {
	Data  []LocationSearchItem `json:"data"`
	Error string               `json:"error,omitempty"`
	// NeedsPermission is true when the Facebook app lacks "Page Public
	// Content Access".
	NeedsPermission bool `json:"needsPermission,omitempty"`
}

// LocationValidateResponse is the Locations.Validate response.
type LocationValidateResponse struct {
	Valid   bool    `json:"valid"`
	ID      *string `json:"id,omitempty"`
	Name    *string `json:"name,omitempty"`
	Address *string `json:"address,omitempty"`
	// Unverified means the id could not be checked right now; the publish
	// step will validate.
	Unverified bool `json:"unverified,omitempty"`
	// Reason explains why the id is not valid / could not be verified.
	Reason *string `json:"reason,omitempty"`
}

// Search calls `GET /locations/search?q=`: search Facebook Places for
// Instagram location tagging. Use a result's ID as LocationID on a post.
func (s *LocationsService) Search(ctx context.Context, query string) (*LocationSearchResponse, error) {
	values := url.Values{}
	values.Set("q", query)
	var out LocationSearchResponse
	if err := s.client.get(ctx, "/locations/search", values, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Validate calls `GET /locations/validate?id=`: check whether a Facebook
// Place id is a valid Instagram location before using it as LocationID.
func (s *LocationsService) Validate(ctx context.Context, id string) (*LocationValidateResponse, error) {
	values := url.Values{}
	values.Set("id", id)
	var out LocationValidateResponse
	if err := s.client.get(ctx, "/locations/validate", values, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
