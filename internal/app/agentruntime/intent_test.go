package agentruntime

import (
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestClassifyDryRunIntent(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-run dry shanghaitech-original"})
	if intent.Kind != IntentSubmitDryRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "workflow.submit_run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.SkillID != "data-to-deployment-lifecycle" {
		t.Fatalf("unexpected skill: %s", intent.SkillID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
}

func TestClassifyAttachmentIntent(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{
		Text:        "帮我看看这个数据",
		Attachments: []channel.Attachment{{ID: "att1"}},
	})
	if intent.Kind != IntentDataIntake {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.SkillID != "channel-data-intake" {
		t.Fatalf("unexpected skill: %s", intent.SkillID)
	}
}

func TestClassifyVisualAttachmentIntentMarksLocalSemanticFastPath(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{
		Text:        "请检查这张异常帧",
		Attachments: []channel.Attachment{{ID: "att1", Name: "frame.png", MediaType: "image/png"}},
	})
	if intent.Kind != IntentDataIntake {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.Metadata["local_semantic_fast_path"] != "true" {
		t.Fatalf("expected local semantic marker for visual attachment, got %+v", intent.Metadata)
	}
}

func TestClassifyRuntimeAboutFastPath(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "你好，你是谁"})
	if intent.Kind != IntentRuntimeAbout {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.Metadata["local_fast_path"] != "true" {
		t.Fatalf("runtime about should be marked as local fast path: %+v", intent.Metadata)
	}
}

func TestClassifyLocateAnythingInstall(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "请帮我下载 HuggingFace nvidia/LocateAnything-3B 模型"})
	if intent.Kind != IntentModelInstall {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "model.download_hf" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
}

func TestClassifyLocateAnythingShanghaiTechTest(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "用 LocateAnything-3B 测试 ShanghaiTech original"})
	if intent.Kind != IntentModelTest {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
}

func TestClassifyVerifyHFJobCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-verify-hf-job nvidia/LocateAnything-3B"})
	if intent.Kind != IntentVerifyHFJob {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "model.verify_hf" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if len(intent.Args) != 1 || intent.Args[0] != "nvidia/LocateAnything-3B" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyTrainingDryRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-train-dry shanghaitech-original detection yolo11n"})
	if intent.Kind != IntentTrainingDryRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "training.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
	if len(intent.Args) != 3 || intent.Args[2] != "yolo11n" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyTrainingRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-train-run shanghaitech-original detection yolo11n"})
	if intent.Kind != IntentTrainingRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "training.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
	if len(intent.Args) != 3 || intent.Args[2] != "yolo11n" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyEvaluationDryRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-eval-dry shanghaitech-original model-1 validation"})
	if intent.Kind != IntentEvaluationDryRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "evaluation.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
	if len(intent.Args) != 3 || intent.Args[1] != "model-1" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyEvaluationRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-eval-run shanghaitech-original model-1 validation"})
	if intent.Kind != IntentEvaluationRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "evaluation.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
	if len(intent.Args) != 3 || intent.Args[1] != "model-1" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyDeploymentDryRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-deploy-dry model-1 local-dry-run python-worker 2"})
	if intent.Kind != IntentDeploymentDryRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "deployment.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if len(intent.Args) != 4 || intent.Args[0] != "model-1" || intent.Args[3] != "2" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyDeploymentRunCommand(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "/bot-deploy-run model-1 local-dry-run python-worker 2"})
	if intent.Kind != IntentDeploymentRun {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "deployment.run" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if len(intent.Args) != 4 || intent.Args[0] != "model-1" || intent.Args[3] != "2" {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
}

func TestClassifyDataIntakePlanningFastPath(t *testing.T) {
	intent := ClassifyIntent(channel.InboundMessage{Text: "请帮我规划 ShanghaiTech 数据接入"})
	if intent.Kind != IntentDataIntake {
		t.Fatalf("unexpected intent: %s", intent.Kind)
	}
	if intent.ToolID != "intake.plan" {
		t.Fatalf("unexpected tool: %s", intent.ToolID)
	}
	if intent.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", intent.DatasetID)
	}
	if intent.Metadata["local_semantic_fast_path"] != "true" {
		t.Fatalf("expected local semantic marker, got %+v", intent.Metadata)
	}
}
