package omnisocials

import (
	"context"
	"net/url"
	"strconv"
)

// InboxService covers the /inbox endpoints: the social inbox (DMs, comments,
// and mentions) across connected platforms. X (Twitter) DMs cost credits: a
// received DM debits 1 credit, and each Reply send debits 2 credits
// up front (auto-refunded if the send fails) — see Reply for the two 402
// error codes this can return.
type InboxService struct {
	client *Client
}

// InboxCursorPagination is the paging block on the inbox list endpoints.
// Unlike the offset-based Pagination, page on by passing NextCursor back as
// the next request's Cursor while HasMore is true. NextCursor is nil on the
// last page.
type InboxCursorPagination struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Limit      int     `json:"limit"`
}

// CursorListResponse is the cursor-paginated list envelope
// `{ "data": [...], "pagination": {...} }` used by the inbox list endpoints.
// Compare ListResponse, which carries the offset-based Pagination.
type CursorListResponse[T any] struct {
	Data       []T                    `json:"data"`
	Pagination *InboxCursorPagination `json:"pagination,omitempty"`
}

// InboxParticipant is a person on the other side of a conversation (or the
// sender of a message).
type InboxParticipant struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Username       string  `json:"username"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
}

// InboxPostRef is the post a comment or mention conversation is attached to
// (nil for DMs).
type InboxPostRef struct {
	ID        *string `json:"id,omitempty"`
	Caption   *string `json:"caption,omitempty"`
	Thumbnail *string `json:"thumbnail,omitempty"`
}

// InboxLastMessage is the latest message preview on a conversation.
type InboxLastMessage struct {
	ID string `json:"id"`
	// Direction is "incoming" (from the participant) or "outgoing" (from you).
	Direction string `json:"direction"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	IsRead    bool   `json:"is_read"`
}

// InboxConversation is one conversation in the social inbox.
type InboxConversation struct {
	ConversationID string `json:"conversation_id"`
	// Platform is the platform identifier, e.g. "instagram", "facebook",
	// "linkedin", "tiktok", "youtube", "x", or "threads".
	Platform string `json:"platform"`
	// Type is the conversation kind: "dm", "comment", or "mention".
	Type        string           `json:"type"`
	Participant InboxParticipant `json:"participant"`
	UnreadCount int              `json:"unread_count"`
	LastMessage InboxLastMessage `json:"last_message"`
	// Post is the related post for comment/mention conversations; nil for DMs.
	Post *InboxPostRef `json:"post,omitempty"`
}

// InboxMessage is a single message within a conversation.
type InboxMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	// Platform is the platform identifier, e.g. "instagram", "facebook",
	// "linkedin", "tiktok", "youtube", "x", or "threads".
	Platform string `json:"platform"`
	// Type is the message kind: "dm", "comment", or "mention".
	Type string `json:"type"`
	// Direction is "incoming" (from the sender) or "outgoing" (from you).
	Direction string `json:"direction"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	IsRead    bool   `json:"is_read"`
	IsReplied bool   `json:"is_replied"`
	// Reaction is the emoji reaction on the message, if any.
	Reaction *string `json:"reaction,omitempty"`
	// ParentCommentID is the parent comment id when this is a threaded comment
	// reply.
	ParentCommentID *string `json:"parent_comment_id,omitempty"`
	// Hidden is set on Threads replies only: true when the reply is hidden on
	// Threads (see Hide). nil (JSON null) for every other platform/message.
	Hidden *bool `json:"hidden,omitempty"`
	// Permalink is a link to the reply or mentioning post on the platform,
	// when known.
	Permalink *string `json:"permalink,omitempty"`
	// Attachment is media on this message, when present. Incoming
	// Instagram/Facebook DM images, videos, voice messages, and story
	// mentions are re-hosted on our CDN so the URL stays valid indefinitely.
	// nil when the message has no media.
	Attachment *InboxAttachment `json:"attachment,omitempty"`
	Sender     InboxParticipant `json:"sender"`
	// Post is the related post for comment/mention messages; nil for DMs.
	Post *InboxPostRef `json:"post,omitempty"`
}

// InboxAttachment is media sent with an inbox message.
type InboxAttachment struct {
	URL string `json:"url"`
	// Type is "image", "video", "audio", or "file".
	Type string `json:"type"`
}

// InboxMarkReadResponse is the Inbox.MarkRead response. Note: unlike the item
// envelope, the fields sit at the top level (not under `data`).
type InboxMarkReadResponse struct {
	ConversationID string `json:"conversation_id"`
	// MarkedRead is the number of messages that were newly marked read.
	MarkedRead int `json:"marked_read"`
}

// InboxListParams filters Inbox.ListConversations. Uses cursor pagination:
// pass a previous response's Pagination.NextCursor as Cursor to page on while
// Pagination.HasMore is true.
type InboxListParams struct {
	// Platform filters by platform: "instagram", "facebook", "linkedin",
	// "tiktok", "youtube", "x", or "threads".
	Platform string
	// Type filters by conversation kind: "dm", "comment", or "mention".
	Type string
	// Unread, when non-nil, filters to conversations with unread messages
	// (omnisocials.Bool(true)) or with none unread (omnisocials.Bool(false)).
	Unread *bool
	// Limit is the max items to return (1-100).
	Limit int
	// Cursor is an opaque cursor from a previous response's
	// Pagination.NextCursor.
	Cursor string
}

// InboxMessagesParams filters Inbox.GetMessages. Uses cursor pagination, same
// shape as InboxListParams.
type InboxMessagesParams struct {
	// Limit is the max items to return.
	Limit int
	// Cursor is an opaque cursor from a previous response's
	// Pagination.NextCursor.
	Cursor string
}

// InboxReplyParams is the request body for Inbox.Reply.
type InboxReplyParams struct {
	// Text is the reply text. Optional when AttachmentURL is set — an
	// attachment-only reply is allowed.
	Text string `json:"text"`
	// AttachmentURL is the public URL of a single media asset to attach
	// (Facebook and Instagram DMs only; other platforms are text-only).
	AttachmentURL string `json:"attachment_url,omitempty"`
	// AttachmentType is the attachment kind: "image", "video", "audio", or
	// "file". Pair it with AttachmentURL.
	AttachmentType string `json:"attachment_type,omitempty"`
}

// ListConversations calls `GET /inbox/conversations`: social inbox
// conversations (DMs, comments, mentions) across connected platforms, newest
// activity first. Filter by Platform, Type, and Unread. Cursor-paginated: pass
// the previous response's Pagination.NextCursor as Cursor to page on while
// Pagination.HasMore is true.
//
// Threads conversations are Type "comment" (replies people leave on your
// Threads posts; ConversationID looks like `threads_comment_<rootPostId>`)
// and "mention" (`threads_mention_<postId>`); there are no Threads DMs.
// Threads inbox is currently rolling out; until Meta approves the permissions
// it is disabled on production, and it needs a Threads connection with the
// reply permission.
func (s *InboxService) ListConversations(ctx context.Context, params *InboxListParams) (*CursorListResponse[InboxConversation], error) {
	query := url.Values{}
	if params != nil {
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.Type != "" {
			query.Set("type", params.Type)
		}
		if params.Unread != nil {
			query.Set("unread", strconv.FormatBool(*params.Unread))
		}
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Cursor != "" {
			query.Set("cursor", params.Cursor)
		}
	}
	var out CursorListResponse[InboxConversation]
	if err := s.client.get(ctx, "/inbox/conversations", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMessages calls `GET /inbox/conversations/:conversationId/messages`: the
// full message thread for one conversation, newest first. Cursor-paginated
// (Limit / Cursor). The id is URL-encoded for you, so pass it exactly as
// returned (LinkedIn ids contain ":" and "()").
func (s *InboxService) GetMessages(ctx context.Context, conversationID string, params *InboxMessagesParams) (*CursorListResponse[InboxMessage], error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Cursor != "" {
			query.Set("cursor", params.Cursor)
		}
	}
	var out CursorListResponse[InboxMessage]
	if err := s.client.get(ctx, "/inbox/conversations/"+url.PathEscape(conversationID)+"/messages", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkRead calls `POST /inbox/conversations/:conversationId/read`: mark every
// message in the conversation as read. Returns the count of messages that were
// newly marked read. The id is URL-encoded for you.
func (s *InboxService) MarkRead(ctx context.Context, conversationID string) (*InboxMarkReadResponse, error) {
	var out InboxMarkReadResponse
	if err := s.client.post(ctx, "/inbox/conversations/"+url.PathEscape(conversationID)+"/read", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Reply calls `POST /inbox/conversations/:conversationId/reply`: send a reply
// into the conversation (a DM message, or a reply to the comment/mention).
// Optionally attach a single media asset by public URL with AttachmentURL +
// AttachmentType. Returns the created outbound message. The id is URL-encoded
// for you.
//
// Replying on an X DM conversation costs 2 prepaid company credits (X's
// per-send fee), debited before the send and automatically refunded if the
// send fails. Two 402 *APIError codes are specific to X: "insufficient_credits"
// (the balance can't cover the 2 credits) and "x_inbox_suspended" (the
// workspace's X inbox auto-suspended when the balance hit zero; top up and
// re-enable X DMs in the dashboard to resume — DMs that arrived while
// suspended are not recovered). Neither code is one of the SDK's typed error
// subclasses, so match them with errors.As against *APIError and check Code.
//
// Threads replies publish as native Threads replies. Threads inbox is
// currently rolling out; until Meta approves the permissions it is disabled
// on production, and it needs a Threads connection with the reply permission:
// a 401 *APIError with Code "reauth_required" means the connection lacks it
// (reconnect Threads).
func (s *InboxService) Reply(ctx context.Context, conversationID string, params *InboxReplyParams) (*ItemResponse[InboxMessage], error) {
	var out ItemResponse[InboxMessage]
	if err := s.client.post(ctx, "/inbox/conversations/"+url.PathEscape(conversationID)+"/reply", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InboxHideParams is the request body for Inbox.Hide.
type InboxHideParams struct {
	// Hide hides the reply when true and unhides it when false.
	Hide bool `json:"hide"`
}

// Hide calls `POST /inbox/messages/:messageId/hide`: hide or unhide a reply
// someone left on one of your Threads posts, as the post owner (Threads only
// for now). hide=true hides, hide=false unhides. Only incoming top-level
// replies can be hidden (Threads does not allow hiding nested replies); the
// message keeps its place in the conversation. Returns the message with
// Hidden flipped. Requires the inbox:write scope. The id is URL-encoded for
// you.
//
// Threads inbox is currently rolling out; until Meta approves the permissions
// it is disabled on production and calls return a clear error. *APIError
// codes: 400 "unsupported_platform" (not an incoming Threads reply, or
// Threads inbox not available yet), 400 "not_hideable" (nested reply or
// Threads refused), 401 "reauth_required" (the Threads connection lacks the
// reply permission; reconnect Threads), 404 "not_found" (message not in this
// workspace) or "account_not_connected" (no Threads account).
func (s *InboxService) Hide(ctx context.Context, messageID string, hide bool) (*ItemResponse[InboxMessage], error) {
	params := &InboxHideParams{Hide: hide}
	var out ItemResponse[InboxMessage]
	if err := s.client.post(ctx, "/inbox/messages/"+url.PathEscape(messageID)+"/hide", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
