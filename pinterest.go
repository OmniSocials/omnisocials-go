package omnisocials

import (
	"context"
	"net/url"
	"strconv"
)

// PinterestService covers the /pinterest endpoints: the product Pins of the
// connected Pinterest account, used to tag products on a Pin.
type PinterestService struct {
	client *Client
}

// PinterestProduct is one product Pin from Pinterest.ListProducts.
type PinterestProduct struct {
	// PinID is the Pin id; use it in the "product_tags" list of a post's
	// Pinterest options.
	PinID       string  `json:"pin_id"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	// Link is the product page the Pin links to.
	Link *string `json:"link,omitempty"`
	// ImageURL is a small image of the product Pin.
	ImageURL *string `json:"image_url,omitempty"`
	// Price, Currency (ISO 4217), Availability (as Pinterest reports it,
	// e.g. "IN_STOCK") and ItemID (the merchant's own product id) are set
	// for the catalog source only.
	Price        *float64 `json:"price,omitempty"`
	Currency     *string  `json:"currency,omitempty"`
	Availability *string  `json:"availability,omitempty"`
	ItemID       *string  `json:"item_id,omitempty"`
}

// PinterestProductGroup is one catalog product group on a
// Pinterest.ListProducts response.
type PinterestProductGroup struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
}

// PinterestProductListParams is the query for Pinterest.ListProducts. Every
// field is optional.
type PinterestProductListParams struct {
	// Source is where to read product Pins from: "catalog" (the Pinterest
	// catalog, needs catalog access) or "pins" (the account's own Pins).
	// Empty uses "catalog" when the connection has catalog access, else
	// "pins".
	Source string
	// ProductGroupID is a product group ID from ProductGroups (catalog
	// source only). Empty uses the group named "All Products", else the
	// first group.
	ProductGroupID string
	// Bookmark is the cursor from the previous response, to get the next
	// page.
	Bookmark string
	// PageSize is the number of products per page, 1 to 100 (catalog source
	// only, default 25).
	PageSize int
}

// PinterestProductsError is the error object on a Pinterest.ListProducts
// response that has no products. Code is one of "pinterest_not_connected"
// (no Pinterest account on the workspace),
// "pinterest_catalog_access_required" (the catalog source was asked but the
// connection has no catalog access; connect the catalog in the composer or
// use the "pins" source), or "platform_error" (Pinterest did not answer or
// refused; Message carries Pinterest's text).
type PinterestProductsError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PinterestProductsResponse is the Pinterest.ListProducts response. Note the
// shape is not the usual data envelope: Products is set on success, Error
// when the list could not be read (no Pinterest account, no catalog access,
// Pinterest did not answer), both with HTTP 200. Validation problems (a
// Source that is not "catalog" or "pins", an unknown ProductGroupID) return
// a 400 *APIError instead.
type PinterestProductsResponse struct {
	Products []PinterestProduct `json:"products,omitempty"`
	// Bookmark is the cursor for the next page; nil on the last page.
	Bookmark *string `json:"bookmark,omitempty"`
	// Source is the source that was read: "catalog" or "pins".
	Source string `json:"source,omitempty"`
	// CatalogAccess is true when the Pinterest connection can read the
	// catalog.
	CatalogAccess bool `json:"catalog_access,omitempty"`
	// NoCatalog is true when the connection has catalog access but the
	// Pinterest account has no catalog yet; Products and ProductGroups are
	// then empty.
	NoCatalog bool `json:"no_catalog,omitempty"`
	// ProductGroups lists the product groups of the account (catalog source
	// only).
	ProductGroups []PinterestProductGroup `json:"product_groups,omitempty"`
	// ProductGroupID is the group these products come from (catalog source
	// only).
	ProductGroupID *string                 `json:"product_group_id,omitempty"`
	Error          *PinterestProductsError `json:"error,omitempty"`
}

// PinterestProductValidateResponse is the Pinterest.ValidateProduct response.
type PinterestProductValidateResponse struct {
	Valid bool `json:"valid"`
	// PinID is the Pin id read from the input (nil when the input is not a
	// Pin id or a Pin link).
	PinID    *string `json:"pin_id,omitempty"`
	Title    *string `json:"title,omitempty"`
	Link     *string `json:"link,omitempty"`
	ImageURL *string `json:"image_url,omitempty"`
	// Unverified means the Pin could not be checked right now; the publish
	// step is the final check.
	Unverified bool `json:"unverified,omitempty"`
	// Reason explains why the Pin is not valid / could not be checked.
	Reason *string `json:"reason,omitempty"`
}

// ListProducts calls `GET /pinterest/products`: list the product Pins of the
// connected Pinterest account. Pass a result's PinID in the "product_tags"
// list of a post's Pinterest options to tag the product on the Pin (max 24
// per Pin). Pinterest only accepts a product Pin that is public, belongs to
// the same account and links to a website that account claimed; products of
// other merchants cannot be tagged.
//
// The "catalog" source reads the Pinterest catalog (with price, currency,
// availability and item id) and needs catalog access, which is given one
// time in the OmniSocials composer (Pinterest options, Add products, Connect
// catalog). The "pins" source reads the account's own Pins and works on
// every connection; one call scans up to 250 Pins, so Products can be empty
// while Bookmark is set (call again with the bookmark). A nil params value
// lets the API pick the source.
func (s *PinterestService) ListProducts(ctx context.Context, params *PinterestProductListParams) (*PinterestProductsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Source != "" {
			query.Set("source", params.Source)
		}
		if params.ProductGroupID != "" {
			query.Set("product_group_id", params.ProductGroupID)
		}
		if params.Bookmark != "" {
			query.Set("bookmark", params.Bookmark)
		}
		if params.PageSize > 0 {
			query.Set("page_size", strconv.Itoa(params.PageSize))
		}
	}
	var out PinterestProductsResponse
	if err := s.client.get(ctx, "/pinterest/products", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ValidateProduct calls `GET /pinterest/products/validate?id=`: check whether
// a Pin can be used as a product tag before creating the post. id is a Pin
// id or a Pin link (https://www.pinterest.com/pin/<id>/; pin.it short links
// do not work).
func (s *PinterestService) ValidateProduct(ctx context.Context, id string) (*PinterestProductValidateResponse, error) {
	values := url.Values{}
	values.Set("id", id)
	var out PinterestProductValidateResponse
	if err := s.client.get(ctx, "/pinterest/products/validate", values, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
