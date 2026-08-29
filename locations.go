package omnisocials

import (
	"context"
	"net/url"
	"strconv"
)

// LocationsService covers the /locations endpoints (Instagram and Threads
// location tagging). Search covers Instagram (Facebook Places); SearchThreads
// covers Threads, whose location ids are not interchangeable with Facebook
// Place ids.
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
// Instagram location tagging. Use a result's ID as LocationID on a post. For
// Threads location tagging use SearchThreads instead (different ids and a
// different response shape).
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

// ThreadsLocationSearchParams is the query for Locations.SearchThreads. Pass
// either Q or the Latitude+Longitude pair.
type ThreadsLocationSearchParams struct {
	// Q is the search text (min 2 characters).
	Q string
	// Latitude (-90 to 90) searches around a point instead of Q. Pair it
	// with Longitude (use omnisocials.Float64).
	Latitude *float64
	// Longitude (-180 to 180). Pair it with Latitude.
	Longitude *float64
}

// ThreadsLocation is one Threads location from Locations.SearchThreads.
type ThreadsLocation struct {
	// ID is the Threads location id; use it as
	// ThreadsPostOptions.LocationID on a post. Threads location ids and
	// Facebook Place ids are not interchangeable.
	ID        string   `json:"id"`
	Name      *string  `json:"name,omitempty"`
	Address   *string  `json:"address,omitempty"`
	City      *string  `json:"city,omitempty"`
	Country   *string  `json:"country,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// ThreadsLocationSearchError is the error object on a degraded Threads
// location search response. Code is one of "not_available" (Threads location
// tagging is not enabled in this environment yet; it is rolling out),
// "threads_not_connected", "threads_reauth_required" (the connection lacks
// the threads_location_tagging permission; reconnect Threads), or
// "platform_error".
type ThreadsLocationSearchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ThreadsLocationSearchResponse is the Locations.SearchThreads response.
// Note the shape differs from the Instagram response: Locations is set on
// success, Error on the degraded path. Validation problems (neither Q nor
// the coordinate pair, Q under 2 characters, coordinates out of range)
// return a 400 *APIError instead.
type ThreadsLocationSearchResponse struct {
	Locations []ThreadsLocation           `json:"locations,omitempty"`
	Error     *ThreadsLocationSearchError `json:"error,omitempty"`
}

// SearchThreads calls `GET /locations/search?platform=threads`: search
// Threads locations by Q, or around a Latitude+Longitude point instead of Q.
// Use a result's ID as ThreadsPostOptions.LocationID on a post. Threads
// location tagging is currently rolling out; until Meta approves the
// permissions it is disabled on production and calls return a clear error
// (see ThreadsLocationSearchError).
func (s *LocationsService) SearchThreads(ctx context.Context, params *ThreadsLocationSearchParams) (*ThreadsLocationSearchResponse, error) {
	values := url.Values{}
	values.Set("platform", "threads")
	if params != nil {
		if params.Q != "" {
			values.Set("q", params.Q)
		}
		if params.Latitude != nil {
			values.Set("latitude", strconv.FormatFloat(*params.Latitude, 'f', -1, 64))
		}
		if params.Longitude != nil {
			values.Set("longitude", strconv.FormatFloat(*params.Longitude, 'f', -1, 64))
		}
	}
	var out ThreadsLocationSearchResponse
	if err := s.client.get(ctx, "/locations/search", values, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
