package omnisocials

import (
	"encoding/json"
	"testing"
)

// The Threads topic serializes as threads.topic_tag; on update Null removes it.
func TestThreadsTopicTagJSON(t *testing.T) {
	create, err := json.Marshal(ThreadsPostOptions{TopicTag: "Build in public"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(create), `{"topic_tag":"Build in public"}`; got != want {
		t.Fatalf("create: got %s, want %s", got, want)
	}

	empty, _ := json.Marshal(ThreadsPostOptions{})
	if got := string(empty); got != `{}` {
		t.Fatalf("no topic must leave the key out, got %s", got)
	}

	update, err := json.Marshal(ThreadsPostOptionsUpdate{TopicTag: Null})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(update), `{"topic_tag":null}`; got != want {
		t.Fatalf("update: got %s, want %s", got, want)
	}
}
