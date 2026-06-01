package qqbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPOutboundSenderPostsSendMsg(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	sender := NewHTTPOutboundSender(OutboundConfig{
		Enabled:     true,
		BaseURL:     server.URL,
		AccessToken: "test-token",
		Timeout:     time.Second,
	})
	err := sender.Send(context.Background(), OneBotReply{
		Action: "send_msg",
		Params: map[string]any{"message_type": "private", "user_id": "10001", "message": "pong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/send_msg" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %s", gotAuth)
	}
	if gotBody["message"] != "pong" {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}

func TestHTTPOutboundSenderDisabledIsNoop(t *testing.T) {
	sender := NewHTTPOutboundSender(OutboundConfig{Enabled: false})
	if err := sender.Send(context.Background(), OneBotReply{Action: "send_msg"}); err != nil {
		t.Fatal(err)
	}
}
