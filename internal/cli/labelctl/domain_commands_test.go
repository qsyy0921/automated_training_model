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
	var autolabelBody map[string]any
	var trainingBody map[string]any
	var evaluationBody map[string]any
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
		case "POST /api/autolabel/jobs":
			if err := json.NewDecoder(r.Body).Decode(&autolabelBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"job": map[string]string{"task_id": "task-auto"}})
		case "POST /api/training/runs":
			if err := json.NewDecoder(r.Body).Decode(&trainingBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"run": map[string]string{"task_id": "task-train"}})
		case "POST /api/evaluation/runs":
			if err := json.NewDecoder(r.Body).Decode(&evaluationBody); err != nil {
				t.Fatal(err)
			}
			writeCLIJSON(t, w, map[string]any{"run": map[string]string{"task_id": "task-eval"}})
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

	if err := runAutoLabel(cfg, []string{"submit", "-dataset", "ds1", "-task-types", "anomaly,label", "-video-ids", "video-1,video-2", "-model-profile", "vlm-default", "-require-review"}); err != nil {
		t.Fatal(err)
	}
	if autolabelBody["dataset_id"] != "ds1" || len(autolabelBody["task_types"].([]any)) != 2 || autolabelBody["dry_run"] != true {
		t.Fatalf("unexpected autolabel body: %+v", autolabelBody)
	}

	if err := runTraining(cfg, []string{"submit", "-dataset", "ds1", "-target-task", "detection", "-model-family", "yolo11n", "-exec-recipe", "default", "-exec", "python", "-exec-arg=-c", "-exec-arg=print('train')", "-exec-cwd", "tmp/train", "-exec-env", "ATM_TEST=1", "-exec-timeout", "30"}); err != nil {
		t.Fatal(err)
	}
	if trainingBody["dataset_id"] != "ds1" || trainingBody["target_task"] != "detection" || trainingBody["model_family"] != "yolo11n" || trainingBody["dry_run"] != false {
		t.Fatalf("unexpected training body: %+v", trainingBody)
	}
	if trainingBody["execution_recipe"] != "default" {
		t.Fatalf("unexpected training execution recipe body: %+v", trainingBody)
	}
	if len(trainingBody["execution_command"].([]any)) != 3 || trainingBody["execution_timeout_seconds"].(float64) != 30 || trainingBody["execution_cwd"] != "tmp/train" {
		t.Fatalf("unexpected training execution body: %+v", trainingBody)
	}

	if err := runEvaluation(cfg, []string{"submit", "-dataset", "ds1", "-model", "model1", "-metrics", "mAP,recall", "-exec", "python", "-exec-arg=-c", "-exec-arg=print('eval')", "-exec-timeout", "31"}); err != nil {
		t.Fatal(err)
	}
	if evaluationBody["dataset_id"] != "ds1" || evaluationBody["model_id"] != "model1" || evaluationBody["dry_run"] != false {
		t.Fatalf("unexpected evaluation body: %+v", evaluationBody)
	}
	if len(evaluationBody["execution_command"].([]any)) != 3 || evaluationBody["execution_timeout_seconds"].(float64) != 31 {
		t.Fatalf("unexpected evaluation execution body: %+v", evaluationBody)
	}

	if err := runDeploy(cfg, []string{"submit", "-model", "model1", "-target", "local", "-replicas", "2", "-exec", "powershell", "-exec-arg=-NoProfile", "-exec-arg=-Command", "-exec-arg=Write-Output hi", "-exec-timeout", "32"}); err != nil {
		t.Fatal(err)
	}
	if deployBody["model_id"] != "model1" || deployBody["target"] != "local" || deployBody["replicas"].(float64) != 2 {
		t.Fatalf("unexpected deploy body: %+v", deployBody)
	}
	if len(deployBody["execution_command"].([]any)) != 4 || deployBody["execution_timeout_seconds"].(float64) != 32 {
		t.Fatalf("unexpected deploy execution body: %+v", deployBody)
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
	if err := runRuntime(cfg, []string{"job-manifest", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"job-lineage", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runModels(cfg, []string{"job-logs", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runModels(cfg, []string{"job-manifest", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogs(cfg, []string{"job", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogs(cfg, []string{"job-manifest", "job1"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"tasks"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"task-logs", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"task-manifest", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"task-lineage", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runRuntime(cfg, []string{"resume-task", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogs(cfg, []string{"tasks"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogs(cfg, []string{"task-manifest", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runDeploy(cfg, []string{"follow-task", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runDeploy(cfg, []string{"task-manifest", "task1"}); err != nil {
		t.Fatal(err)
	}
	if err := runTraining(cfg, []string{"resume-task", "task1"}); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"POST /api/datasets/register-folder",
		"POST /api/autolabel/jobs",
		"POST /api/training/runs",
		"POST /api/evaluation/runs",
		"POST /api/models/register",
		"POST /api/deployments",
		"POST /api/channels/qq/test-message",
		"GET /api/runtime/model-jobs/job1/logs",
		"GET /api/runtime/model-jobs/job1/manifest",
		"GET /api/runtime/model-jobs/job1/lineage",
		"GET /api/tasks",
		"GET /api/tasks/task1/logs",
		"GET /api/tasks/task1/manifest",
		"GET /api/tasks/task1/lineage",
		"GET /api/tasks/task1/logs/stream",
		"POST /api/tasks/task1/resume",
	} {
		want := 1
		if key == "GET /api/runtime/model-jobs/job1/logs" {
			want = 3
		}
		if key == "GET /api/runtime/model-jobs/job1/manifest" {
			want = 3
		}
		if key == "GET /api/runtime/model-jobs/job1/lineage" {
			want = 1
		}
		if key == "GET /api/tasks" {
			want = 2
		}
		if key == "GET /api/tasks/task1/logs" {
			want = 1
		}
		if key == "GET /api/tasks/task1/manifest" {
			want = 3
		}
		if key == "GET /api/tasks/task1/lineage" {
			want = 1
		}
		if key == "POST /api/tasks/task1/resume" {
			want = 2
		}
		if seen[key] != want {
			t.Fatalf("expected %s %d times, got %d", key, want, seen[key])
		}
	}
}

func TestRunAutoLabelSupportsExecutionFlags(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		writeCLIJSON(t, w, map[string]any{"job": map[string]string{"task_id": "task-auto"}})
	}))
	defer server.Close()

	if err := runAutoLabel(Config{addr: server.URL}, []string{"submit", "-dataset", "ds1", "-task-types", "label", "-exec-recipe", "default", "-exec-cwd", "tmp/auto", "-exec-env", "AUTO=1", "-exec-timeout", "45"}); err != nil {
		t.Fatal(err)
	}
	if body["execution_recipe"] != "default" || body["execution_cwd"] != "tmp/auto" || body["execution_timeout_seconds"].(float64) != 45 {
		t.Fatalf("unexpected autolabel execution body: %+v", body)
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
