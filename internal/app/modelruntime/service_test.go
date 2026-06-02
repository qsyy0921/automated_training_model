package modelruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareHFModelRequestUsesDefaultsInsideDataLake(t *testing.T) {
	req, err := PrepareHFModelRequest(map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if req.RepoID != "nvidia/LocateAnything-3B" {
		t.Fatalf("unexpected repo: %s", req.RepoID)
	}
	if !strings.HasSuffix(filepath.ToSlash(req.LocalDir), "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B") {
		t.Fatalf("unexpected local dir: %s", req.LocalDir)
	}
	if !strings.HasSuffix(filepath.ToSlash(req.Manifest), "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json") {
		t.Fatalf("unexpected manifest: %s", req.Manifest)
	}
}

func TestPrepareHFModelRequestRejectsPathEscape(t *testing.T) {
	_, err := PrepareHFModelRequest(map[string]string{
		"repo_id":   "nvidia/LocateAnything-3B",
		"local_dir": "../outside",
	}, false)
	if err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestPrepareLocateAnythingSmokeRequestUsesAllowedDefaults(t *testing.T) {
	req, err := PrepareLocateAnythingSmokeRequest(map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(req.ModelDir), "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B") {
		t.Fatalf("unexpected model dir: %s", req.ModelDir)
	}
	if !strings.Contains(filepath.ToSlash(req.DataRoot), "data_lake/raw/datasets/shanghaitech/original") {
		t.Fatalf("unexpected data root: %s", req.DataRoot)
	}
	if !strings.Contains(filepath.ToSlash(req.Output), "data_lake/catalog/models/nvidia_LocateAnything-3B.smoke.json") {
		t.Fatalf("unexpected output: %s", req.Output)
	}
}

func TestDownloadRequiresApprovalCanBeEnabled(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL", "true")
	svc := NewService()
	if !svc.DownloadRequiresApproval(map[string]string{}) {
		t.Fatal("expected approval requirement")
	}
	if svc.DownloadRequiresApproval(map[string]string{"approved": "true"}) {
		t.Fatal("approved request should pass")
	}
}

func TestParseLocateAnythingSmokeOutputNormalizesPartial(t *testing.T) {
	status, modelLoad, realInference := ParseLocateAnythingSmokeOutput([]byte(`log line
{"status":"partial","completed":{"model_load":true,"real_inference":false}}`))
	if status != "ok" || modelLoad != "true" || realInference != "false" {
		t.Fatalf("unexpected parse result: %s %s %s", status, modelLoad, realInference)
	}
}
