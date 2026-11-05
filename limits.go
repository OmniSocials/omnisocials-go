package omnisocials

import (
	"context"
	"net/url"
)

// LimitsService covers GET /limits: what OmniSocials cannot do, why, and
// what to do instead (platform rules, features not built yet, features that
// work in the OmniSocials app only). Read it before you tell a person that
// something is not possible, so you can give the reason (Says) and the way
// around it (Instead). When the person still wants a Requestable entry,
// send it with FeatureRequests.Create and its LimitID. No scope needed:
// every key may call it.
//
// Error codes: 400 invalid_platform, invalid_area.
type LimitsService struct {
	client *Client
}

// LimitKinds are the values of Limit.Kind. "platform" (the platform does
// not allow it) and "by_design" (OmniSocials does not do it on purpose)
// cannot change through a request; the others can.
var LimitKinds = []string{"platform", "by_design", "not_built", "beta", "dashboard_only", "needs_setup"}

// LimitAreas are the values of Limit.Area, LimitListParams.Area and
// FeatureRequestCreateParams.Area.
var LimitAreas = []string{"posts", "media", "inbox", "analytics", "automations", "accounts", "ads", "team", "other"}

// Limit is something OmniSocials cannot do, why, and what to do instead.
type Limit struct {
	// ID is for example "tiktok.delete". Pass it as
	// FeatureRequestCreateParams.LimitID.
	ID string `json:"id"`
	// Kind is one of LimitKinds.
	Kind string `json:"kind"`
	// KindLabel is the kind in words, e.g. "The platform does not allow it".
	KindLabel string `json:"kind_label"`
	// Area is one of LimitAreas.
	Area string `json:"area"`
	// Platforms are the platforms it is about; empty = every channel.
	Platforms []string `json:"platforms"`
	Title     string   `json:"title"`
	// Says is the reason, to tell the person.
	Says string `json:"says"`
	// Instead is what to do instead, nil when there is nothing.
	Instead *string `json:"instead"`
	// Requestable is whether a feature request can name it. False for the
	// "platform" and "by_design" kinds.
	Requestable bool `json:"requestable"`
}

// LimitsResponse is the response of Limits.List.
type LimitsResponse struct {
	Data []Limit `json:"data"`
	// Kinds is a label per kind ("platform": "The platform does not allow
	// it", ...).
	Kinds map[string]string `json:"kinds"`
}

// LimitListParams filters Limits.List.
type LimitListParams struct {
	// Platform keeps the entries for one platform (entries for every
	// channel stay in): instagram, facebook, linkedin, linkedin_page,
	// youtube, tiktok, x, pinterest, bluesky, threads, mastodon,
	// google_business, telegram, discord, reddit, snapchat.
	Platform string
	// Area is one of LimitAreas.
	Area string
}

// List calls `GET /limits`: the things OmniSocials cannot do, optionally
// for one platform or area. params may be nil.
func (s *LimitsService) List(ctx context.Context, params *LimitListParams) (*LimitsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.Area != "" {
			query.Set("area", params.Area)
		}
	}
	var out LimitsResponse
	if err := s.client.get(ctx, "/limits", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
