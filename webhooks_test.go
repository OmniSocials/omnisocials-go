package omnisocials

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
