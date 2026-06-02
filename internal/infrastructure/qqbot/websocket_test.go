package qqbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRunWebSocketClientReadsEventAndWritesReply(t *testing.T) {
	upgrader := websocket.Upgrader{}
	gotReply := make(chan OneBotReply, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		err = conn.WriteJSON(map[string]any{
			"post_type":    "message",
			"message_type": "private",
			"message_id":   "m1",
			"user_id":      "10001",
			"message":      "/bot-ping",
			"raw_message":  "/bot-ping",
		})
		if err != nil {
			t.Error(err)
			return
		}
		var reply OneBotReply
		if err := conn.ReadJSON(&reply); err != nil {
			t.Error(err)
			return
		}
		gotReply <- reply
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunWebSocketClient(ctx, WebSocketConfig{
			Enabled:        true,
			URL:            "ws" + strings.TrimPrefix(server.URL, "http"),
			AccountID:      "default",
			ReconnectDelay: time.Millisecond,
		}, func(ctx context.Context, event OneBotEvent) (OneBotReply, bool, error) {
			msg := NormalizeEvent(event, "default")
			if msg.Text != "/bot-ping" {
				t.Errorf("unexpected normalized text: %q", msg.Text)
			}
			return OneBotReply{Action: "send_msg", Params: map[string]any{"message_type": "private", "user_id": msg.SenderID, "message": "pong"}}, true, nil
		})
	}()

	select {
	case reply := <-gotReply:
		if reply.Action != "send_msg" || reply.Params["message"] != "pong" {
			raw, _ := json.Marshal(reply)
			t.Fatalf("unexpected reply: %s", string(raw))
		}
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for websocket reply")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for websocket client shutdown")
	}
}

func TestRunWebSocketClientDisabledIsNoop(t *testing.T) {
	if err := RunWebSocketClient(context.Background(), WebSocketConfig{Enabled: false}, nil); err != nil {
		t.Fatal(err)
	}
}
