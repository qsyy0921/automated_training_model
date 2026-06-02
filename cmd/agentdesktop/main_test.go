package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunDashboardRequestsDesktopStatus(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/desktop/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeTestJSON(t, w, map[string]any{
			"desktop": map[string]any{
				"status":  "ready",
				"profile": "local-desktop",
				"gateway": "127.0.0.1:7870",
				"runtime": "automated-training-agent-runtime",
			},
		})
	}))
	defer server.Close()

	if err := run(config{addr: server.URL, token: "secret"}, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("expected bearer token, got %q", gotAuth)
	}
}

func TestRunSendPostsThroughRuntimeAdapter(t *testing.T) {
	var gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/channels/qq/test-message" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		gotText, _ = payload["text"].(string)
		writeTestJSON(t, w, map[string]any{
			"reply": map[string]any{"text": "pong"},
		})
	}))
	defer server.Close()

	if err := run(config{addr: server.URL}, []string{"send", "/bot-ping"}); err != nil {
		t.Fatal(err)
	}
	if gotText != "/bot-ping" {
		t.Fatalf("expected /bot-ping, got %q", gotText)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
