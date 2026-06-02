package labelctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDomainCommandsUseGatewayEndpoints(t *testing.T) {
	seen := map[string]int{}
	var datasetBody map[string]any
	var modelBody map[string]any
	var deployBody map[string]any
	var desktopText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing auth header for %s", r.URL.Path)
		}
		seen[r.Method+" "+r.URL.Path]++
		switch r.Method + " " + r.URL.Path {
		case "POST /api/datasets/register-folder":
			if err := json.NewDecoder(r.Body).Decode(&datasetBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"dataset": map[string]string{"id": "ds1"}})
		case "POST /api/models/register":
			if err := json.NewDecoder(r.Body).Decode(&modelBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"model": map[string]string{"id": "model1"}})
		case "POST /api/deployments":
			if err := json.NewDecoder(r.Body).Decode(&deployBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"deployment": map[string]string{"task_id": "task1"}})
		case "POST /api/channels/qq/test-message":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			desktopText, _ = payload["text"].(string)
			writeCLIJSON(t, w, map[string]any{"reply": map[string]string{"text": "pong"}})
		default:
			writeCLIJSON(t, w, map[string]any{"ok": true})
		}
	}))
	defer server.Close()

	oldToken := cliGatewayToken
	cliGatewayToken = "secret"
	defer func() { cliGatewayToken = oldToken }()
	cfg := Config{addr: server.URL, token: "secret"}

	if err := runDataset(cfg, []string{"register-folder", "-name", "demo", "-merge-root", "F:\\data\\merge"}); err != nil {
		t.Fatal(err)
	}
	if datasetBody["name"] != "demo" || datasetBody["merge_root"] != "F:\\data\\merge" {
		t.Fatalf("unexpected dataset body: %+v", datasetBody)
	}

	if err := runModels(cfg, []string{"register", "-name", "locate", "-artifact-uri", "data_lake/models/x", "-tags", "vision, smoke"}); err != nil {
		t.Fatal(err)
	}
	if modelBody["name"] != "locate" || modelBody["artifact_uri"] != "data_lake/models/x" {
		t.Fatalf("unexpected model body: %+v", modelBody)
	}

	if err := runDeploy(cfg, []string{"submit", "-model", "model1", "-target", "local", "-replicas", "2"}); err != nil {
		t.Fatal(err)
	}
	if deployBody["model_id"] != "model1" || deployBody["target"] != "local" || deployBody["replicas"].(float64) != 2 {
		t.Fatalf("unexpected deploy body: %+v", deployBody)
	}

	if err := runDesktop(cfg, []string{"send", "/bot-ping"}); err != nil {
		t.Fatal(err)
	}
	if desktopText != "/bot-ping" {
		t.Fatalf("unexpected desktop text: %q", desktopText)
	}
	if err := runRuntime(cfg, []string{"job-logs", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runModels(cfg, []string{"job-logs", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogs(cfg, []string{"job", "job1"}); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"POST /api/datasets/register-folder",
		"POST /api/models/register",
		"POST /api/deployments",
		"POST /api/channels/qq/test-message",
		"GET /api/runtime/model-jobs/job1/logs",
	} {
		want := 1
		if key == "GET /api/runtime/model-jobs/job1/logs" {
			want = 3
		}
		if seen[key] != want {
			t.Fatalf("expected %s %d times, got %d", key, want, seen[key])
		}
	}
}

func TestRunDoctorProbesCoreEndpoints(t *testing.T) {
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path]++
		writeCLIJSON(t, w, map[string]any{"ok": true})
	}))
	defer server.Close()

	if err := runDoctor(Config{addr: server.URL}, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/healthz", "/api/runtime/status", "/api/channels", "/api/channels/qq/status", "/api/desktop/status", "/api/models", "/api/datasets"} {
		if seen[path] != 1 {
			t.Fatalf("expected doctor to probe %s, got %d", path, seen[path])
		}
	}
}

func writeCLIJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
