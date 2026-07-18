package omnisocials

import (
	"context"
	"net/url"
)

// AudioService covers the /audio endpoints (Instagram Reels licensed audio).
type AudioService struct {
	client *Client
}

// AudioTrack is one track from Audio.Search.
type AudioTrack struct {
	// AudioID is the Meta audio id; use it as `instagram.audio_id` on a
	// reel post.
	AudioID string  `json:"audio_id"`
	Title   *string `json:"title,omitempty"`
	Artist  *string `json:"artist,omitempty"`
	// DurationMS is the track duration in milliseconds.
	DurationMS *int `json:"duration_ms,omitempty"`
	// AudioType is "music" or "original_sound".
	AudioType string  `json:"audio_type"`
	CoverURL  *string `json:"cover_url,omitempty"`
	// PreviewURL is a temporary URL (~1.5 days); never persist it.
	PreviewURL *string `json:"preview_url,omitempty"`
	IGUsername *string `json:"ig_username,omitempty"`
}

// AudioSearchResponse is the Audio.Search response. Note: unlike the usual
// envelope, Error here is a plain string set when search is unavailable
// (e.g. no Facebook account connected), with Data then empty.
type AudioSearchResponse struct {
	Data  []AudioTrack `json:"data"`
	Error string       `json:"error,omitempty"`
	// NeedsFacebook is true when the workspace needs a Facebook connection
	// linked to the Instagram account.
	NeedsFacebook bool `json:"needsFacebook,omitempty"`
}

// Search calls `GET /audio/search?q=&type=`: search Meta's licensed audio
// catalog for Instagram Reels. An empty query returns trending audio;
// audioType is "music" (server default) or "original_sound" (empty strings
// omit the params). Use a result's AudioID as `instagram.audio_id` on a reel
// post (with optional `instagram.audio_volume` / `instagram.video_volume`,
// integers 0-100). PreviewURL is temporary (~1.5 days); never persist it.
func (s *AudioService) Search(ctx context.Context, query string, audioType string) (*AudioSearchResponse, error) {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if audioType != "" {
		values.Set("type", audioType)
	}
	var out AudioSearchResponse
	if err := s.client.get(ctx, "/audio/search", values, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
