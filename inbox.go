package omnisocials

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
// (nil for DMs), so a reply can be drafted with the post in view.
type InboxPostRef struct {
	ID      *string `json:"id,omitempty"`
	Caption *string `json:"caption,omitempty"`
	// Thumbnail is the image URL of the post (or the video's cover).
	Thumbnail *string `json:"thumbnail,omitempty"`
	// URL is the public link to the post when the platform provides one
	// (Instagram, Facebook, YouTube, TikTok, LinkedIn, Threads); nil otherwise.
	URL *string `json:"url,omitempty"`
	// MediaType is the platform's own media label when known (e.g. "IMAGE",
	// "VIDEO", "CAROUSEL_ALBUM" on Instagram); nil otherwise.
	MediaType *string `json:"media_type,omitempty"`
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
	// Hidden is set on comments and mentions: true when the comment is hidden
	// on the platform (see Hide), false when it is not. nil (JSON null) for
	// DMs, which have no notion of hidden.
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
	// Unanswered, when omnisocials.Bool(true), returns only conversations that
	// still need an answer: the customer's latest DM has no reply after it
	// (Instagram/Facebook DMs within the 24-hour messaging window only), or a
	// comment/mention that has not been replied to and is not hidden. Replies
	// typed in the native apps count as answers (they are mirrored into the
	// inbox). Read state is ignored here; use Next for a work queue.
	Unanswered *bool
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
	// IncludeNext, when true, makes the response also carry Next (the next
	// conversation that needs an answer, the same object Next returns in
	// Data, using its default queue order and filters; nil when nothing is
	// waiting) and Remaining. Saves the extra call when working through the
	// inbox.
	IncludeNext bool `json:"include_next,omitempty"`
}

// InboxReplyResponse is the Inbox.Reply response: the created outbound
// message under Data, plus Next and Remaining when
// InboxReplyParams.IncludeNext was set.
type InboxReplyResponse struct {
	Data    InboxMessage `json:"data"`
	Message string       `json:"message,omitempty"`
	// Next is only set when IncludeNext was true: the next conversation that
	// needs an answer, or nil when nothing is waiting.
	Next *InboxNextUnanswered `json:"next,omitempty"`
	// Remaining is only set when IncludeNext was true: unanswered items still
	// waiting after Next (capped at 500).
	Remaining *int `json:"remaining,omitempty"`
}

// ListConversations calls `GET /inbox/conversations`: social inbox
// conversations (DMs, comments, mentions) across connected platforms, newest
// activity first. Filter by Platform, Type, Unread, and Unanswered.
// Cursor-paginated: pass
// the previous response's Pagination.NextCursor as Cursor to page on while
// Pagination.HasMore is true.
//
// Threads conversations are Type "comment" (replies people leave on your
// Threads posts; ConversationID looks like `threads_comment_<rootPostId>`)
// and "mention" (`threads_mention_<postId>`); there are no Threads DMs.
// The Threads inbox needs a Threads connection with the reply permissions;
// connections made before those permissions existed must be reconnected once.
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
		if params.Unanswered != nil {
			query.Set("unanswered", strconv.FormatBool(*params.Unanswered))
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
// Threads replies publish as native Threads replies. The Threads inbox needs
// a Threads connection with the reply permission: a 401 *APIError with Code
// "reauth_required" means the connection lacks it (connected before it
// existed; reconnect Threads).
//
// Set IncludeNext to also get Next (the next conversation that needs an
// answer, the same object Next returns in Data, using its default queue order
// and filters; nil when nothing is waiting) and Remaining in the response.
// Saves the extra call when working through the inbox.
func (s *InboxService) Reply(ctx context.Context, conversationID string, params *InboxReplyParams) (*InboxReplyResponse, error) {
	var out InboxReplyResponse
	if err := s.client.post(ctx, "/inbox/conversations/"+url.PathEscape(conversationID)+"/reply", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InboxHideParams is the request body for Inbox.Hide.
type InboxHideParams struct {
	// Hide hides the comment when true and unhides it when false.
	Hide bool `json:"hide"`
}

// Hide calls `POST /inbox/messages/:messageId/hide`: hide or unhide a comment
// someone left on one of your posts, on the platform, as the post owner.
// Facebook, Instagram, TikTok, YouTube and Threads comments (Threads: incoming
// top-level replies only; Threads does not allow hiding nested replies).
// hide=true hides, hide=false unhides. On YouTube, hide sets the comment's
// moderation status to rejected, which removes it and its replies from public
// view; unhide publishes it again. The message keeps its place in the
// conversation and Hidden flips on the returned message; a hidden comment no
// longer counts as unanswered. Requires the inbox:write scope. The account
// must have been connected with the moderation permission (Facebook
// pages_manage_engagement, Instagram instagram_business_manage_comments). The
// id is URL-encoded for you.
//
// *APIError codes: 400 "unsupported_platform" (not an incoming comment on a
// supported platform), 400 "not_hideable" (Threads nested reply, or Threads
// refused), 401 "reauth_required" (the Threads reply permission or the TikTok
// comments authorization is missing or expired), 403 "reconnect_required"
// (the account was connected without the comment-moderation permission;
// reconnect it in the dashboard), 404 "not_found" (message not in this
// workspace) or "account_not_connected", 429 "quota_exceeded" (YouTube's
// daily API quota is used up; retry after midnight Pacific), 502
// "platform_error" (the platform rejected the call). The Threads inbox needs
// a Threads connection with the reply permissions; a connection made before
// those permissions existed answers 401 "reauth_required" until reconnected.
func (s *InboxService) Hide(ctx context.Context, messageID string, hide bool) (*ItemResponse[InboxMessage], error) {
	params := &InboxHideParams{Hide: hide}
	var out ItemResponse[InboxMessage]
	if err := s.client.post(ctx, "/inbox/messages/"+url.PathEscape(messageID)+"/hide", jsonBody(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InboxDeletedMessage is the Data of the Inbox.DeleteMessage response.
type InboxDeletedMessage struct {
	// ID is the deleted message id.
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	// RemovedReplyIDs are the inbox ids of replies removed together with the
	// comment.
	RemovedReplyIDs []string `json:"removed_reply_ids"`
}

// DeleteMessage calls `DELETE /inbox/messages/:messageId`: delete a comment
// someone left on one of your posts, on the platform and from the inbox.
// Facebook, Instagram and TikTok comments only: YouTube's API does not let a
// channel delete other people's comments, hide those instead (see Hide).
// Replies under the deleted comment go with it (the platforms cascade the
// delete and the inbox mirrors that); their inbox ids come back as
// RemovedReplyIDs. A comment that is already gone on the platform is still
// removed from the inbox. This cannot be undone. Requires the inbox:write
// scope. The id is URL-encoded for you.
//
// *APIError codes: 400 "unsupported_platform" (not an incoming Facebook,
// Instagram or TikTok comment), 401 "reauth_required" (the TikTok comments
// authorization expired), 403 "reconnect_required" (the account was connected
// without the comment-moderation permission; reconnect it in the dashboard),
// 404 "not_found" (message not in this workspace) or "account_not_connected",
// 502 "platform_error" (the platform rejected the call).
func (s *InboxService) DeleteMessage(ctx context.Context, messageID string) (*ItemResponse[InboxDeletedMessage], error) {
	var out ItemResponse[InboxDeletedMessage]
	if err := s.client.do(ctx, http.MethodDelete, "/inbox/messages/"+url.PathEscape(messageID), nil, nil, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InboxNextParams filters Inbox.Next. All fields are optional.
type InboxNextParams struct {
	// Platform limits the queue to one platform: "instagram", "facebook",
	// "linkedin", "tiktok", "youtube", "x", or "threads".
	Platform string
	// Type limits the queue to one kind: "dm", "comment", or "mention".
	Type string
	// Order is "oldest" (the default: the item that has waited longest first)
	// or "newest" (the most recent).
	Order string
	// IncludeRead also serves items that were marked read but never answered.
	// By default only unread items are served, so marking a conversation read
	// is the durable way to skip it.
	IncludeRead bool
	// Exclude lists conversation ids to leave out of this call (a
	// session-local skip; up to 100). Sent comma-separated.
	Exclude []string
}

// InboxNextUnanswered is the next conversation that needs an answer, with
// everything needed to draft the reply.
type InboxNextUnanswered struct {
	Conversation InboxConversation `json:"conversation"`
	// Message is the unanswered incoming message itself: the customer's
	// latest DM, or the specific comment. Its ID is what Hide and
	// DeleteMessage take; its ConversationID is what Reply takes.
	Message InboxMessage `json:"message"`
	// Messages is the conversation so far, oldest first (the most recent 50
	// messages for long DM threads).
	Messages []InboxMessage `json:"messages"`
}

// InboxNextResponse is the Inbox.Next response.
type InboxNextResponse struct {
	// Data is the next item, or nil when nothing is waiting.
	Data *InboxNextUnanswered `json:"data"`
	// Remaining is the number of unanswered items still waiting after this
	// one (capped at 500). 0 when Data is nil.
	Remaining int `json:"remaining"`
}

// Next calls `GET /inbox/next`: the next conversation that needs an answer,
// a work queue for answering the inbox. Returns the oldest (by default) item
// that still needs a reply, together with its conversation so far and the
// post it belongs to, so a reply can be drafted from one call. An item needs
// an answer when it is the customer's latest DM with no reply after it
// (Instagram/Facebook DMs within the 24-hour messaging window only, since
// Meta refuses replies outside it), or a comment/mention that has not been
// replied to and is not hidden. Replies typed in the native apps count as
// answers (they are mirrored into the inbox), so a thread a colleague
// answered on their phone is not served again. Instagram mentions are skipped
// (no reply path). Looks at the last 30 days of activity. Requires the
// inbox:read scope.
//
// Only unread items are served by default: marking a conversation read
// (MarkRead) is how to skip one for good; set IncludeRead to include
// read-but-unanswered items. Exclude is a session-local skip. Data is nil
// when nothing is waiting; Remaining counts the unanswered items still waiting
// after this one (capped at 500). To chain the queue, set
// InboxReplyParams.IncludeNext on Reply and it returns the next item in the
// same response. *APIError codes: 400 "validation_error" (unknown platform,
// type or order).
func (s *InboxService) Next(ctx context.Context, params *InboxNextParams) (*InboxNextResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Platform != "" {
			query.Set("platform", params.Platform)
		}
		if params.Type != "" {
			query.Set("type", params.Type)
		}
		if params.Order != "" {
			query.Set("order", params.Order)
		}
		if params.IncludeRead {
			query.Set("include_read", "true")
		}
		if len(params.Exclude) > 0 {
			query.Set("exclude", strings.Join(params.Exclude, ","))
		}
	}
	var out InboxNextResponse
	if err := s.client.get(ctx, "/inbox/next", query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
