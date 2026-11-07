package omnisocials

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// signLikeDispatcher signs exactly like the backend webhookDispatcher.js:
// HMAC-SHA256 hex over "{timestamp}.{rawBody}", header "t=<ts>,v1=<hex>".
func signLikeDispatcher(secret string, timestamp int64, rawBody string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, rawBody)))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

const testEventBody = `{"id":"evt_123","type":"post.published","created_at":"2026-07-14T10:00:00.000Z","data":{"post_id":"42","workspace_id":"7","status":"published","targets":[{"platform":"instagram","status":"published","native_post_id":"1789"}]}}`

func TestVerifyWebhookSignatureRoundTrip(t *testing.T) {
	secret := "whsec_test_secret"
	timestamp := time.Now().Unix()
	header := signLikeDispatcher(secret, timestamp, testEventBody)

	event, err := VerifyWebhookSignature([]byte(testEventBody), header, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected valid signature to verify, got error: %v", err)
	}
	if event["type"] != "post.published" {
		t.Fatalf("expected event type post.published, got %v", event["type"])
	}
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected event data object, got %T", event["data"])
	}
	if data["post_id"] != "42" {
		t.Fatalf("expected post_id 42, got %v", data["post_id"])
	}
}

// For 24 hours after a secret rotation a delivery also carries
// X-OmniSocials-Signature-Previous: the same scheme, signed with the OLD
// secret. The new secret fails on it; the old secret verifies it.
func TestVerifyWebhookSignaturePreviousHeaderWithOldSecret(t *testing.T) {
	newSecret, oldSecret := "whsec_new_secret", "whsec_old_secret"
	timestamp := time.Now().Unix()
	current := signLikeDispatcher(newSecret, timestamp, testEventBody)
	previous := signLikeDispatcher(oldSecret, timestamp, testEventBody)

	if _, err := VerifyWebhookSignature([]byte(testEventBody), previous, newSecret, 0); err == nil {
		t.Fatal("the new secret must not verify the Previous header")
	}
	if _, err := VerifyWebhookSignature([]byte(testEventBody), current, oldSecret, 0); err == nil {
		t.Fatal("the old secret must not verify the current header")
	}
	event, err := VerifyWebhookSignature([]byte(testEventBody), previous, oldSecret, 0)
	if err != nil {
		t.Fatalf("the Previous header must verify with the old secret: %v", err)
	}
	if event["id"] != "evt_123" {
		t.Fatalf("unexpected event: %v", event)
	}
}

func TestWebhookWorkspaceIDDecodes(t *testing.T) {
	var list ListResponse[Webhook]
	raw := `{"data":[
		{"id":"5","url":"https://example.com/a","events":["post.published"],"workspace_id":1000401,"is_active":true,"last_triggered_at":null,"failure_count":0,"last_failure":null,"created_at":"2026-11-06T10:00:00.000Z"},
		{"id":"6","url":"https://example.com/b","events":["post.failed"],"workspace_id":null,"is_active":false,"last_triggered_at":null,"failure_count":0,
		 "last_failure":{"at":"2026-11-06T11:00:00.000Z","status":null,"error":"The person who set up this webhook no longer has access to the organisation.","attempts":0},"created_at":"2026-11-06T10:00:00.000Z"}]}`
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Data[0].WorkspaceID == nil || *list.Data[0].WorkspaceID != 1000401 {
		t.Fatalf("expected workspace_id 1000401, got %v", list.Data[0].WorkspaceID)
	}
	off := list.Data[1]
	if off.WorkspaceID != nil {
		t.Fatalf("expected nil workspace_id for an organisation-wide webhook, got %d", *off.WorkspaceID)
	}
	if off.IsActive || off.LastFailure == nil || off.LastFailure.Status != nil || off.LastFailure.Attempts != 0 || off.LastFailure.Error == "" {
		t.Fatalf("unexpected switched-off webhook: %+v", off)
	}
}

func TestVerifyWebhookSignatureRejectedEventKeepsApproval(t *testing.T) {
	secret := "whsec_test_secret"
	body := `{"id":"evt_456","type":"post.rejected","created_at":"2026-10-02T09:00:05.000Z","data":{"post_id":"123456","workspace_id":789,"status":"rejected","post_type":"Post","scheduled_at":"2026-10-03T14:00:00.000Z","published_at":null,"targets":[],"approval":{"status":"rejected","decided_by":"c4a09e1d","reason":"Wrong product photo"}}}`
	header := signLikeDispatcher(secret, time.Now().Unix(), body)

	event, err := VerifyWebhookSignature([]byte(body), header, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected valid signature to verify, got error: %v", err)
	}
	if event["type"] != "post.rejected" {
		t.Fatalf("expected event type post.rejected, got %v", event["type"])
	}
	data, ok := event["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected event data object, got %T", event["data"])
	}
	approval, ok := data["approval"].(map[string]any)
	if !ok {
		t.Fatalf("expected data.approval object, got %T", data["approval"])
	}
	if approval["status"] != "rejected" || approval["decided_by"] != "c4a09e1d" || approval["reason"] != "Wrong product photo" {
		t.Fatalf("unexpected approval object: %v", approval)
	}
	if targets, ok := data["targets"].([]any); !ok || len(targets) != 0 {
		t.Fatalf("expected empty targets, got %v", data["targets"])
	}
}

func TestVerifyWebhookSignatureAdStatusChangedEvent(t *testing.T) {
	secret := "whsec_test_secret"
	body := `{"id":"evt_789","type":"ad.status_changed","created_at":"2026-07-13T10:00:05Z","data":{"campaign_id":42,"workspace_id":"456","name":"Boost: Summer launch post","platform":"meta","campaign_type":"boost","status":"active","previous_status":"pending_review","external_campaign_id":"23851234567890123","budget":{"type":"daily","amount":10,"currency":"EUR"}}}`
	header := signLikeDispatcher(secret, time.Now().Unix(), body)

	event, err := VerifyWebhookSignature([]byte(body), header, secret, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected valid signature to verify, got error: %v", err)
	}
	if event["type"] != "ad.status_changed" {
		t.Fatalf("expected event type ad.status_changed, got %v", event["type"])
	}

	// The data is the campaign, not a post: decode it into the typed struct.
	var delivery struct {
		Type string                   `json:"type"`
		Data AdStatusChangedEventData `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &delivery); err != nil {
		t.Fatalf("decode the delivery: %v", err)
	}
	ad := delivery.Data
	if ad.CampaignID != 42 || ad.WorkspaceID.String() != "456" || ad.Status != "active" ||
		ad.PreviousStatus == nil || *ad.PreviousStatus != "pending_review" ||
		ad.Platform == nil || *ad.Platform != "meta" || ad.CampaignType == nil || *ad.CampaignType != "boost" ||
		ad.ExternalCampaignID == nil || *ad.ExternalCampaignID != "23851234567890123" ||
		ad.Budget == nil || ad.Budget.Amount != 10 || ad.Budget.Currency != "EUR" {
		t.Fatalf("unexpected ad.status_changed data: %+v", ad)
	}

	// The campaign id may also arrive as a string, and a field may be null.
	var stringID AdStatusChangedEventData
	if err := json.Unmarshal([]byte(`{"campaign_id":"42","workspace_id":null,"status":"rejected","previous_status":null,"budget":null}`), &stringID); err != nil {
		t.Fatalf("decode with a string id: %v", err)
	}
	if stringID.CampaignID != 42 || stringID.PreviousStatus != nil || stringID.Budget != nil {
		t.Fatalf("unexpected data: %+v", stringID)
	}
}

func TestVerifyWebhookSignatureTamperedBodyFails(t *testing.T) {
	secret := "whsec_test_secret"
	timestamp := time.Now().Unix()
	header := signLikeDispatcher(secret, timestamp, testEventBody)

	tampered := []byte(`{"id":"evt_123","type":"post.published","data":{"post_id":"HACKED"}}`)
	_, err := VerifyWebhookSignature(tampered, header, secret, 5*time.Minute)
	if err == nil {
		t.Fatal("expected tampered body to fail verification")
	}
	var verr *WebhookVerificationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *WebhookVerificationError, got %T: %v", err, err)
	}
}

func TestVerifyWebhookSignatureStaleTimestampFails(t *testing.T) {
	secret := "whsec_test_secret"
	stale := time.Now().Add(-time.Hour).Unix()
	header := signLikeDispatcher(secret, stale, testEventBody)

	_, err := VerifyWebhookSignature([]byte(testEventBody), header, secret, 5*time.Minute)
	if err == nil {
		t.Fatal("expected stale timestamp to fail verification")
	}
	var verr *WebhookVerificationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *WebhookVerificationError, got %T: %v", err, err)
	}
}

func TestVerifyWebhookSignatureWrongSecretFails(t *testing.T) {
	timestamp := time.Now().Unix()
	header := signLikeDispatcher("whsec_right", timestamp, testEventBody)

	if _, err := VerifyWebhookSignature([]byte(testEventBody), header, "whsec_wrong", 5*time.Minute); err == nil {
		t.Fatal("expected wrong secret to fail verification")
	}
}

func TestVerifyWebhookSignatureMalformedHeaderFails(t *testing.T) {
	for _, header := range []string{"garbage", "t=notanumber,v1=abc", "v1=deadbeef", "t=1700000000"} {
		if _, err := VerifyWebhookSignature([]byte(testEventBody), header, "whsec_test", 5*time.Minute); err == nil {
			t.Fatalf("expected malformed header %q to fail verification", header)
		}
	}
}

func TestVerifyWebhookSignatureEmptyInputsFail(t *testing.T) {
	timestamp := time.Now().Unix()
	header := signLikeDispatcher("whsec_test", timestamp, testEventBody)

	if _, err := VerifyWebhookSignature([]byte(testEventBody), header, "", 0); err == nil {
		t.Fatal("expected missing secret to fail")
	}
	if _, err := VerifyWebhookSignature([]byte(testEventBody), "", "whsec_test", 0); err == nil {
		t.Fatal("expected missing signature to fail")
	}
	if _, err := VerifyWebhookSignature(nil, header, "whsec_test", 0); err == nil {
		t.Fatal("expected missing payload to fail")
	}
}

func TestVerifyWebhookSignatureToleratesExtraPairsAndMultipleV1(t *testing.T) {
	secret := "whsec_test_secret"
	timestamp := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", timestamp, testEventBody)))
	good := hex.EncodeToString(mac.Sum(nil))
	header := fmt.Sprintf("t=%d,v0=ignored,v1=%s,v1=%s", timestamp, "deadbeef", good)

	if _, err := VerifyWebhookSignature([]byte(testEventBody), header, secret, 5*time.Minute); err != nil {
		t.Fatalf("expected header with extra pairs and one matching v1 to verify, got: %v", err)
	}
}

func TestVerifyWebhookSignatureDefaultTolerance(t *testing.T) {
	secret := "whsec_test_secret"

	// Fresh delivery passes with tolerance <= 0 (falls back to the default).
	fresh := time.Now().Unix()
	header := signLikeDispatcher(secret, fresh, testEventBody)
	if _, err := VerifyWebhookSignature([]byte(testEventBody), header, secret, 0); err != nil {
		t.Fatalf("expected fresh delivery to verify with default tolerance, got: %v", err)
	}

	// A 10-minute-old delivery fails the 5-minute default.
	old := time.Now().Add(-10 * time.Minute).Unix()
	header = signLikeDispatcher(secret, old, testEventBody)
	if _, err := VerifyWebhookSignature([]byte(testEventBody), header, secret, 0); err == nil {
		t.Fatal("expected 10-minute-old delivery to fail the default tolerance")
	}
}

func TestVerifyWebhookSignatureNonJSONPayloadFails(t *testing.T) {
	secret := "whsec_test_secret"
	timestamp := time.Now().Unix()
	body := "not json at all"
	header := signLikeDispatcher(secret, timestamp, body)

	_, err := VerifyWebhookSignature([]byte(body), header, secret, 5*time.Minute)
	if err == nil {
		t.Fatal("expected non-JSON payload to fail even with a valid signature")
	}
	var verr *WebhookVerificationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *WebhookVerificationError, got %T: %v", err, err)
	}
}
