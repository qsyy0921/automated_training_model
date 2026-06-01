package qqbot

import (
	"encoding/json"
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestNormalizePrivateTextEvent(t *testing.T) {
	raw := []byte(`{"post_type":"message","message_type":"private","message_id":101,"user_id":12345,"raw_message":"/bot-ping","message":"/bot-ping"}`)
	var event OneBotEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	msg := NormalizeEvent(event, "default")
	if msg.Peer.Kind != channel.PeerKindDirect {
		t.Fatalf("unexpected peer kind: %s", msg.Peer.Kind)
	}
	if msg.Text != "/bot-ping" {
		t.Fatalf("unexpected text: %q", msg.Text)
	}
	if msg.SenderID != "12345" {
		t.Fatalf("unexpected sender: %s", msg.SenderID)
	}
}

func TestNormalizeGroupImageEvent(t *testing.T) {
	raw := []byte(`{"post_type":"message","message_type":"group","message_id":"m1","group_id":999,"user_id":12345,"message":[{"type":"at","data":{"qq":"111"}},{"type":"text","data":{"text":"看这个"}},{"type":"image","data":{"file":"img1.jpg","url":"http://example/img1.jpg"}}]}`)
	var event OneBotEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	msg := NormalizeEvent(event, "default")
	if msg.Peer.Kind != channel.PeerKindGroup || msg.Peer.ID != "999" {
		t.Fatalf("unexpected peer: %+v", msg.Peer)
	}
	if !msg.Mentioned {
		t.Fatal("expected mentioned")
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].MediaType != "image" {
		t.Fatalf("unexpected attachments: %+v", msg.Attachments)
	}
}

func TestBuildSendMessage(t *testing.T) {
	reply := BuildSendMessage(channel.OutboundMessage{
		Peer: channel.Peer{Kind: channel.PeerKindGroup, ID: "999"},
		Text: "pong",
	})
	if reply.Action != "send_msg" {
		t.Fatalf("unexpected action: %s", reply.Action)
	}
	if reply.Params["group_id"] != "999" {
		t.Fatalf("unexpected target: %+v", reply.Params)
	}
}
