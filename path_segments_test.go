package omnisocials

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// An id stays inside one encoded path segment, and an id of "", "." or ".."
// is refused before anything is sent: URL normalisation would drop that
// segment or climb a level and reach another endpoint of the same API key.
func TestHostileIDStaysInOneSegment(t *testing.T) {
	var gotPath string
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		fmt.Fprint(w, `{"data":{}}`)
	}))
	if _, err := client.Posts.Publish(context.Background(), "../webhooks/wh_fake/rotate-secret#"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if want := "/posts/..%2Fwebhooks%2Fwh_fake%2Frotate-secret%23/publish"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestDotAndEmptyIDsAreRefused(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"data":{}}`)
	}), WithMaxRetries(0))
	ctx := context.Background()
	for _, bad := range []string{"", ".", ".."} {
		attempts := map[string]func() error{
			"Posts.Delete": func() error { return client.Posts.Delete(ctx, bad) },
			"Webhooks.RotateSecret": func() error {
				_, err := client.Webhooks.RotateSecret(ctx, bad)
				return err
			},
			"Inbox.MarkRead": func() error {
				_, err := client.Inbox.MarkRead(ctx, bad)
				return err
			},
			"Ads.Drafts.Get": func() error {
				_, err := client.Ads.Drafts.Get(ctx, bad)
				return err
			},
		}
		for name, call := range attempts {
			err := call()
			if err == nil || !strings.Contains(err.Error(), "invalid request path") {
				t.Fatalf("%s(%q): expected an invalid request path error, got %v", name, bad, err)
			}
		}
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected no request for a refused id, got %d", got)
	}
}
