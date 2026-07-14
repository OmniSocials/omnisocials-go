package omnisocials

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is the default maximum allowed age of a webhook
// delivery's timestamp (replay protection).
const DefaultWebhookTolerance = 5 * time.Minute

// VerifyWebhookSignature verifies an OmniSocials webhook delivery
// (Stripe-style scheme) and returns the parsed event object on success.
//
// payload must be the RAW request body, exactly as received. Do not parse and
// re-serialize it first: the signature is computed over the raw bytes.
// signature is the value of the X-OmniSocials-Signature header, in the form
// `t=<unix>,v1=<hex>` where the hex value is an HMAC-SHA256 of
// "{timestamp}.{rawBody}" using the webhook's signing secret. secret is the
// webhook's signing secret (shown once on create / rotate-secret).
//
// tolerance is the maximum allowed age of the timestamp; deliveries older
// than that are rejected as possible replays. Pass a non-positive tolerance
// to use DefaultWebhookTolerance (5 minutes).
//
// The comparison is constant-time. On any failure the returned error is a
// *WebhookVerificationError.
func VerifyWebhookSignature(payload []byte, signature, secret string, tolerance time.Duration) (map[string]any, error) {
	if secret == "" {
		return nil, &WebhookVerificationError{Message: "no webhook secret provided"}
	}
	if signature == "" {
		return nil, &WebhookVerificationError{
			Message: "no signature provided; expected the X-OmniSocials-Signature header value",
		}
	}
	if payload == nil {
		return nil, &WebhookVerificationError{Message: "no payload provided"}
	}
	if tolerance <= 0 {
		tolerance = DefaultWebhookTolerance
	}

	// Parse `t=<unix>,v1=<hex>` (tolerate extra/unknown pairs and multiple v1).
	var timestampRaw string
	var candidates []string
	for _, part := range strings.Split(signature, ",") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "t":
			timestampRaw = value
		case "v1":
			candidates = append(candidates, value)
		}
	}

	timestamp, err := strconv.ParseInt(timestampRaw, 10, 64)
	if timestampRaw == "" || err != nil {
		return nil, &WebhookVerificationError{
			Message: "unable to extract timestamp from signature header; expected format: t=<unix>,v1=<hex>",
		}
	}
	if len(candidates) == 0 {
		return nil, &WebhookVerificationError{
			Message: "unable to extract v1 signature from signature header; expected format: t=<unix>,v1=<hex>",
		}
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestampRaw))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	matched := false
	for _, candidate := range candidates {
		// hmac.Equal is constant-time; check every candidate without an
		// early exit.
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			matched = true
		}
	}
	if !matched {
		return nil, &WebhookVerificationError{
			Message: "webhook signature verification failed: no v1 signature matches the expected signature",
		}
	}

	age := time.Now().Unix() - timestamp
	if age > int64(tolerance/time.Second) {
		return nil, &WebhookVerificationError{
			Message: fmt.Sprintf(
				"webhook timestamp is outside the allowed tolerance of %s (event is %ds old); possible replay",
				tolerance, age,
			),
		}
	}

	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, &WebhookVerificationError{
			Message: "webhook payload is not valid JSON (did you pass the raw request body?)",
		}
	}
	return event, nil
}
