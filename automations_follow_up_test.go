package omnisocials

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

const followUpAutomation = `{"data":{"id":"41","type":"comment_to_dm","platform":"instagram","status":"active",
"trigger":{"kinds":["comment"],"scope":"post","keywords":["guide"],"match":"word"},
"actions":{"dm":{"text":"Here is the guide"},"public_reply":{"enabled":false,"variations":[]},"follow_gate":null,
"follow_up":{"enabled":true,"text":"Did it help?","delay_hours":23,"button":{"title":"Book a call","url":"https://example.com/call"},
"tap":{"text":"Tap the button and I'll send you the link 👇","button":"Send me the link"}}},
"limits":{"max_sends":null,"stop_at":null},
"stats":{"matched":5,"sent":4,"skipped":1,"failed":0,"followed":0,"follow_ups_sent":3,"follow_ups_skipped":1,"last_event_at":null}}}`

func TestAutomationFollowUpUpdateBody(t *testing.T) {
	ctx := context.Background()
	client, got := recordRequests(t, followUpAutomation, http.StatusOK)
	res, err := client.Automations.Update(ctx, "41", &AutomationUpdateParams{
		Actions: &AutomationActions{
			DM: AutomationDM{Text: "Here is the guide"},
			FollowUp: &AutomationFollowUp{
				Enabled:    Bool(true),
				Text:       "Did it help?",
				DelayHours: 23,
				Button:     &FollowUpButton{Title: "Book a call", URL: "https://example.com/call"},
				Tap:        &FollowUpTap{Button: "Send it"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Automations.Update: %v", err)
	}
	req := (*got)[0]
	if req.method != http.MethodPatch || req.path != "/automations/41" {
		t.Fatalf("unexpected request: %+v", req)
	}
	followUp, _ := req.body["actions"].(map[string]any)["follow_up"].(map[string]any)
	if followUp["enabled"] != true || followUp["text"] != "Did it help?" || followUp["delay_hours"] != float64(23) {
		t.Fatalf("unexpected follow_up: %v", followUp)
	}
	if button, _ := followUp["button"].(map[string]any); button["title"] != "Book a call" || button["url"] != "https://example.com/call" {
		t.Fatalf("unexpected button: %v", followUp["button"])
	}
	if tap, _ := followUp["tap"].(map[string]any); tap["button"] != "Send it" || tap["text"] != nil {
		t.Fatalf("unexpected tap (an empty Text must be left out): %v", followUp["tap"])
	}

	f := res.Data.Actions.FollowUp
	if f == nil || f.Enabled == nil || !*f.Enabled || f.DelayHours != 23 || f.Button == nil || f.Button.URL != "https://example.com/call" {
		t.Fatalf("unexpected follow_up in the response: %+v", f)
	}
	if f.Tap == nil || f.Tap.Button != "Send me the link" {
		t.Fatalf("unexpected tap in the response: %+v", f.Tap)
	}
	if res.Data.Stats.FollowUpsSent != 3 || res.Data.Stats.FollowUpsSkipped != 1 {
		t.Fatalf("unexpected stats: %+v", res.Data.Stats)
	}
}

func TestAutomationFollowUpRemoveButton(t *testing.T) {
	body, err := json.Marshal(AutomationActions{
		DM:       AutomationDM{Text: "Here is the guide"},
		FollowUp: &AutomationFollowUp{RemoveButton: true},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fu, _ := sent["follow_up"].(map[string]any)
	if value, ok := fu["button"]; !ok || value != nil {
		t.Fatalf(`RemoveButton must send "button": null, got %s`, body)
	}
	if len(fu) != 1 {
		t.Fatalf("RemoveButton alone must send only the button, got %s", body)
	}

	// Without RemoveButton a nil Button is left out (keeps the stored one).
	body, _ = json.Marshal(AutomationFollowUp{Enabled: Bool(false)})
	if string(body) != `{"enabled":false}` {
		t.Fatalf("unexpected body: %s", body)
	}
	// A Button wins over RemoveButton.
	body, _ = json.Marshal(AutomationFollowUp{RemoveButton: true, Button: &FollowUpButton{Title: "Go", URL: "https://example.com"}})
	if string(body) != `{"button":{"title":"Go","url":"https://example.com"}}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestAutomationEventFollowUp(t *testing.T) {
	ctx := context.Background()
	reply := `{"data":[
{"id":"1","automation_id":"41","kind":"comment","outcome":"waiting","reason":null,"reason_label":null,
 "person":{"id":"p1","username":"anna"},"follow_gate":{"state":"waiting","asked_at":"2026-11-16T10:00:00Z","taps":0,"followed":false,"checked":false,"after_ask":false,"mode":"tap"},
 "follow_up":null,"created_at":"2026-11-16T10:00:00Z","sent_at":null},
{"id":"2","automation_id":"41","kind":"comment","outcome":"sent","reason":null,"reason_label":null,
 "person":{"id":"p2","username":"ben"},"follow_gate":null,
 "follow_up":{"state":"skipped","due_at":"2026-11-17T09:00:00Z","sent_at":null,"reason":"window_closed","reason_label":"No follow-up","platform_message_id":null},
 "created_at":"2026-11-16T10:00:00Z","sent_at":"2026-11-16T10:00:05Z"}],
"counts":{"waiting":1,"sent":1},"pagination":{"total":2,"limit":25,"offset":0}}`
	client, _ := recordRequests(t, reply, http.StatusOK)
	events, err := client.Automations.ListEvents(ctx, "41", nil)
	if err != nil {
		t.Fatalf("Automations.ListEvents: %v", err)
	}
	if len(events.Data) != 2 {
		t.Fatalf("unexpected events: %+v", events.Data)
	}
	gate := events.Data[0].FollowGate
	if gate == nil || gate.Mode != "tap" || events.Data[0].FollowUp != nil {
		t.Fatalf("unexpected first event: %+v", events.Data[0])
	}
	fu := events.Data[1].FollowUp
	if fu == nil || fu.State != "skipped" || fu.Reason == nil || *fu.Reason != "window_closed" || fu.DueAt == nil || fu.SentAt != nil {
		t.Fatalf("unexpected follow_up: %+v", fu)
	}
}
