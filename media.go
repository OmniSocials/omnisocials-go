package omnisocials

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"bytes"
)

// MediaService covers the /media endpoints.
type MediaService struct {
	client *Client
}

// MediaItem is a Media Library item as returned by the API.
type MediaItem struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	// Type is "image" or "video".
	Type string `json:"type"`
	// Name is the human-readable label (falls back to the filename).
	Name     *string `json:"name,omitempty"`
	Filename string  `json:"filename,omitempty"`
	FolderID *string `json:"folder_id,omitempty"`
	// Size is a human-formatted size string (e.g. "2.50 MB"), not a byte
	// count.
	Size string `json:"size,omitempty"`
	// Status is "ready" for normal uploads. Large async URL ingests start as
	// "processing" (poll until "ready" before using in a post) and become
	// "failed" when the source could not be fetched/validated.
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// PDFInfo describes a rasterized PDF upload.
type PDFInfo struct {
	TotalPages    int  `json:"total_pages"`
	RenderedPages int  `json:"rendered_pages"`
	Truncated     bool `json:"truncated"`
}

// MediaUploadResponse is the envelope returned by the upload endpoints. A PDF
// is rasterized into one image slide per page (max 20): Data mirrors the
// first slide (back-compat) while Slides + MediaIDs carry the whole carousel
// in page order. Pass ALL of MediaIDs to Posts.Create; on LinkedIn the slides
// post as a native swipeable document, elsewhere as an image carousel.
type MediaUploadResponse struct {
	Data MediaItem `json:"data"`
	// Compatibility lists connected platforms that would reject this file,
	// with reasons.
	Compatibility map[string]any `json:"compatibility,omitempty"`
	Message       string         `json:"message,omitempty"`
	// Slides, MediaIDs, and PDF are only present for PDF uploads.
	Slides   []MediaItem `json:"slides,omitempty"`
	MediaIDs []string    `json:"media_ids,omitempty"`
	PDF      *PDFInfo    `json:"pdf,omitempty"`
}

// MediaListParams filters Media.List.
type MediaListParams struct {
	// Limit is the max items to return (default 20, max 100).
	Limit int
	// Offset is the number of items to skip (default 0).
	Offset int
	// Search is a free-text search over name + filename.
	Search string
	// FolderID filters by folder. Use "root" (or "null") for unfiled items.
	FolderID string
}

// MediaUploadParams is the input for Media.Upload (multipart).
type MediaUploadParams struct {
	// File is the file contents. Required.
	File io.Reader
	// Filename is sent with the multipart part (defaults to "upload.bin").
	Filename string
	// Name is an optional human-readable label so the asset is findable by
	// name later.
	Name string
	// Folder is a folder name to file the asset under (created at the top
	// level when missing).
	Folder string
	// FolderID is the id of an existing folder to file the asset under.
	FolderID string
}

// MediaUploadFromURLParams is the request body for Media.UploadFromURL.
type MediaUploadFromURLParams struct {
	// URL is a publicly accessible http(s) URL. Files up to 1GB are
	// supported.
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
	// Name is an optional human-readable label so the asset is findable by
	// name later.
	Name string `json:"name,omitempty"`
	// Folder is a folder name to file the asset under (created at the top
	// level when missing).
	Folder string `json:"folder,omitempty"`
	// FolderID is the id of an existing folder to file the asset under.
	FolderID string `json:"folder_id,omitempty"`
}

// MediaUploadFromBase64Params is the request body for Media.UploadFromBase64.
type MediaUploadFromBase64Params struct {
	// Data is the base64-encoded file data (without a data URI prefix).
	Data string `json:"data"`
	// MimeType is e.g. "image/jpeg", "image/png", "application/pdf".
	MimeType string `json:"mime_type"`
	Filename string `json:"filename,omitempty"`
	Name     string `json:"name,omitempty"`
	Folder   string `json:"folder,omitempty"`
	FolderID string `json:"folder_id,omitempty"`
}

// CreateUploadURLResponse is the Media.CreateUploadURL response: a one-time
// presigned upload URL for large files (up to 1GB). POST the file as
// multipart form data (field name "file") to UploadURL within
// ExpiresInSeconds; no auth headers are needed on that second request.
type CreateUploadURLResponse struct {
	UploadURL        string `json:"upload_url"`
	UploadToken      string `json:"upload_token"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	Method           string `json:"method,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	Instructions     string `json:"instructions,omitempty"`
	Hint             string `json:"hint,omitempty"`
}

// MediaCheckParams is the request body for Media.Check. Provide one of: a
// public URL, an existing MediaID, or SizeBytes + Mime.
type MediaCheckParams struct {
	// URL is the public URL of the file to preflight.
	URL string `json:"url,omitempty"`
	// MediaID is the id of an already-uploaded Library item.
	MediaID string `json:"media_id,omitempty"`
	// SizeBytes is the raw size in bytes (pair with Mime).
	SizeBytes int64 `json:"size_bytes,omitempty"`
	// Mime is the MIME type (pair with SizeBytes).
	Mime string `json:"mime,omitempty"`
}

// MediaUpdateParams is the request body for Media.Update.
type MediaUpdateParams struct {
	// Name is the human-readable label. Use omnisocials.String("") to clear
	// it.
	Name *string `json:"name,omitempty"`
	// FolderID moves the file: a folder id string, or omnisocials.Null to
	// move it to the root ("All media").
	FolderID any `json:"folder_id,omitempty"`
}

// List calls `GET /media`: the media library, newest first.
func (s *MediaService) List(ctx context.Context, params *MediaListParams) (*ListResponse[MediaItem], error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset > 0 {
			query.Set("offset", strconv.Itoa(params.Offset))
		}
		if params.Search != "" {
			query.Set("search", params.Search)
		}
		if params.FolderID != "" {
			query.Set("folder_id", params.FolderID)
		}
	}
	var out ListResponse[MediaItem]
	if err := s.client.get(ctx, "/media", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get calls `GET /media/:id`: fetch a single media item.
func (s *MediaService) Get(ctx context.Context, id string) (*ItemResponse[MediaItem], error) {
	var out ItemResponse[MediaItem]
	if err := s.client.get(ctx, "/media/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Upload calls `POST /media/upload`: upload a file as multipart form data
// (max 50MB for images, 100MB request cap overall; use UploadFromURL or
// CreateUploadURL for larger videos, up to 1GB). The reader is buffered in
// memory so the request can be retried. A PDF is rasterized into image slides
// and returned as a carousel (Slides + MediaIDs).
func (s *MediaService) Upload(ctx context.Context, params *MediaUploadParams) (*MediaUploadResponse, error) {
	if params == nil || params.File == nil {
		return nil, errors.New("omnisocials: media upload requires MediaUploadParams.File")
	}

	filename := params.Filename
	if filename == "" {
		filename = "upload.bin"
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, &ConnectionError{Message: "failed to build multipart body: " + err.Error(), Err: err}
	}
	if _, err := io.Copy(part, params.File); err != nil {
		return nil, &ConnectionError{Message: "failed to read upload file: " + err.Error(), Err: err}
	}
	if params.Name != "" {
		if err := writer.WriteField("name", params.Name); err != nil {
			return nil, &ConnectionError{Message: "failed to build multipart body: " + err.Error(), Err: err}
		}
	}
	if params.Folder != "" {
		if err := writer.WriteField("folder", params.Folder); err != nil {
			return nil, &ConnectionError{Message: "failed to build multipart body: " + err.Error(), Err: err}
		}
	}
	if params.FolderID != "" {
		if err := writer.WriteField("folder_id", params.FolderID); err != nil {
			return nil, &ConnectionError{Message: "failed to build multipart body: " + err.Error(), Err: err}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, &ConnectionError{Message: "failed to finalize multipart body: " + err.Error(), Err: err}
	}

	var out MediaUploadResponse
	if err := s.client.do(ctx, http.MethodPost, "/media/upload", nil, buf.Bytes(), writer.FormDataContentType(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadFromURL calls `POST /media/upload-from-url`: the server fetches a
// public URL (files up to 1GB; large videos finish processing in the
// background and come back with status "processing").
func (s *MediaService) UploadFromURL(ctx context.Context, params *MediaUploadFromURLParams) (*MediaUploadResponse, error) {
	var out MediaUploadResponse
	if err := s.client.post(ctx, "/media/upload-from-url", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadFromBase64 calls `POST /media/upload-from-base64`: upload
// base64-encoded file data.
func (s *MediaService) UploadFromBase64(ctx context.Context, params *MediaUploadFromBase64Params) (*MediaUploadResponse, error) {
	var out MediaUploadResponse
	if err := s.client.post(ctx, "/media/upload-from-base64", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateUploadURL calls `POST /media/create-upload-url`: mint a one-time
// presigned upload URL for large files (up to 1GB). POST the file as
// multipart form data (field name "file") to the returned UploadURL within
// ExpiresInSeconds; no auth headers are needed on that second request.
func (s *MediaService) CreateUploadURL(ctx context.Context) (*CreateUploadURLResponse, error) {
	var out CreateUploadURLResponse
	if err := s.client.post(ctx, "/media/create-upload-url", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Check calls `POST /media/check`: preflight a file's compatibility with the
// workspace's connected platforms BEFORE uploading. Provide one of: a public
// URL, an existing MediaID, or SizeBytes + Mime.
func (s *MediaService) Check(ctx context.Context, params *MediaCheckParams) (map[string]any, error) {
	var out map[string]any
	if err := s.client.post(ctx, "/media/check", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update calls `PATCH /media/:id`: rename a file and/or move it into a
// folder.
func (s *MediaService) Update(ctx context.Context, id string, params *MediaUpdateParams) (*ItemResponse[MediaItem], error) {
	var out ItemResponse[MediaItem]
	if err := s.client.patch(ctx, "/media/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /media/:id`: delete a media file (204 on success).
// Fails with 409 media_in_use when the file is attached to a scheduled or
// publishing post.
func (s *MediaService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/media/"+url.PathEscape(id))
}
