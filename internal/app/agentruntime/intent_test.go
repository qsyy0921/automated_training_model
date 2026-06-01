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
