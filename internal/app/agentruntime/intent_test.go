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
