package omnisocials

import (
	"context"
	"net/url"
)

// FoldersService covers the /folders endpoints (Media Library folders).
type FoldersService struct {
	client *Client
}

// Folder is a Media Library folder.
type Folder struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
	// ItemCount is the number of media items directly inside the folder.
	ItemCount int    `json:"item_count,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// FolderCreateParams is the request body for Folders.Create.
type FolderCreateParams struct {
	Name string `json:"name"`
	// ParentID nests the folder under an existing folder.
	ParentID string `json:"parent_id,omitempty"`
}

// FolderUpdateParams is the request body for Folders.Update.
type FolderUpdateParams struct {
	// Name renames the folder.
	Name string `json:"name,omitempty"`
	// ParentID moves the folder: a folder id string, or omnisocials.Null to
	// move it to the top level. Cycles are rejected.
	ParentID any `json:"parent_id,omitempty"`
}

// List calls `GET /folders`: all folders as a flat list (build the tree via
// ParentID).
func (s *FoldersService) List(ctx context.Context) (*ListResponse[Folder], error) {
	var out ListResponse[Folder]
	if err := s.client.get(ctx, "/folders", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create calls `POST /folders`: create a folder (optionally nested under
// ParentID).
func (s *FoldersService) Create(ctx context.Context, params *FolderCreateParams) (*ItemResponse[Folder], error) {
	var out ItemResponse[Folder]
	if err := s.client.post(ctx, "/folders", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update calls `PATCH /folders/:id`: rename (Name) and/or move (ParentID, or
// omnisocials.Null for the top level).
func (s *FoldersService) Update(ctx context.Context, id string, params *FolderUpdateParams) (*ItemResponse[Folder], error) {
	var out ItemResponse[Folder]
	if err := s.client.patch(ctx, "/folders/"+url.PathEscape(id), jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete calls `DELETE /folders/:id`: delete a folder (204 on success). Media
// is preserved: files move to the root and subfolders move up to the deleted
// folder's parent.
func (s *FoldersService) Delete(ctx context.Context, id string) error {
	return s.client.del(ctx, "/folders/"+url.PathEscape(id))
}
