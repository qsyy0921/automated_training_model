package agentruntime

import (
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type IntentKind string

const (
	IntentUnknown       IntentKind = "unknown"
	IntentHealthCheck   IntentKind = "health_check"
	IntentIdentifyActor IntentKind = "identify_actor"
	IntentRuntimeStatus IntentKind = "runtime_status"
	IntentListRuns      IntentKind = "list_runs"
	IntentSubmitDryRun  IntentKind = "submit_dry_run"
	IntentDataIntake    IntentKind = "data_intake"
	IntentRuntimeAbout  IntentKind = "runtime_about"
	IntentModelInstall  IntentKind = "model_install"
	IntentModelTest     IntentKind = "model_test"
	IntentVerifyHFJob   IntentKind = "verify_hf_job"
	IntentChat          IntentKind = "chat"
)

type Intent struct {
	Kind       IntentKind        `json:"kind"`
	RawText    string            `json:"raw_text,omitempty"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	SkillID    string            `json:"skill_id,omitempty"`
	ToolID     string            `json:"tool_id,omitempty"`
	MCPServer  string            `json:"mcp_server,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func ClassifyIntent(msg channel.InboundMessage) Intent {
	text := strings.TrimSpace(msg.Text)
	if len(msg.Attachments) > 0 {
		return Intent{
			Kind:       IntentDataIntake,
			RawText:    text,
			SkillID:    "channel-data-intake",
			ToolID:     "intake.plan",
			Confidence: 1,
		}
	}
	if text == "" {
		return Intent{Kind: IntentUnknown, Confidence: 1}
	}
	if runtimeAboutText(text) {
		return Intent{
			Kind:       IntentRuntimeAbout,
			RawText:    text,
			SkillID:    "runtime-self-description",
			Confidence: 0.95,
			Metadata:   map[string]string{"local_fast_path": "true"},
		}
	}
	if modelTestText(text) {
		return Intent{
			Kind:       IntentModelTest,
			RawText:    text,
			DatasetID:  inferredDatasetID(text),
			SkillID:    "model-validation",
			ToolID:     "model.smoke_locateanything",
			Confidence: 0.9,
		}
	}
	if modelInstallText(text) {
		return Intent{
			Kind:       IntentModelInstall,
			RawText:    text,
			SkillID:    "huggingface-model-downloader",
			ToolID:     "model.download_hf",
			Confidence: 0.9,
		}
	}
	if dataIntakeText(text) {
		return Intent{
			Kind:       IntentDataIntake,
			RawText:    text,
			DatasetID:  inferredDatasetID(text),
			SkillID:    "channel-data-intake",
			ToolID:     "intake.plan",
			Confidence: 0.88,
			Metadata:   map[string]string{"local_semantic_fast_path": "true"},
		}
	}
	if !strings.HasPrefix(text, "/") {
		return Intent{
			Kind:       IntentChat,
			RawText:    text,
			SkillID:    "agent-conversation",
			ToolID:     "llm.plan",
			Confidence: 0.7,
		}
	}
	fields := strings.Fields(text)
	command := strings.ToLower(fields[0])
	intent := Intent{RawText: text, Command: command, Args: fields[1:], Confidence: 1}
	switch command {
	case "/bot-ping":
		intent.Kind = IntentHealthCheck
		intent.ToolID = "runtime.health"
	case "/bot-me":
		intent.Kind = IntentIdentifyActor
		intent.ToolID = "runtime.identify_actor"
	case "/bot-status":
		intent.Kind = IntentRuntimeStatus
		intent.ToolID = "runtime.status"
	case "/bot-runs":
		intent.Kind = IntentListRuns
		intent.ToolID = "workflow.list_runs"
	case "/bot-run":
		if len(fields) >= 2 && strings.ToLower(fields[1]) == "dry" {
			intent.Kind = IntentSubmitDryRun
			intent.SkillID = "data-to-deployment-lifecycle"
			intent.ToolID = "workflow.submit_run"
			if len(fields) >= 3 {
				intent.DatasetID = fields[2]
			}
		}
	case "/bot-verify-hf-job":
		intent.Kind = IntentVerifyHFJob
		intent.SkillID = "huggingface-model-downloader"
		intent.ToolID = "model.verify_hf"
	default:
		intent.Kind = IntentUnknown
	}
	return intent
}

func runtimeAboutText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	normalized = strings.ReplaceAll(normalized, " ", "")
	if normalized == "你好" || normalized == "你哈" || normalized == "hello" || normalized == "hi" {
		return true
	}
	for _, marker := range []string{
		"你是谁",
		"你是什么",
		"介绍一下你自己",
		"你能做什么",
		"当前能力",
		"runtime能做什么",
		"agent能做什么",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func modelInstallText(text string) bool {
	normalized := strings.ToLower(text)
	hasModel := strings.Contains(normalized, "huggingface") || strings.Contains(normalized, "locateanything") || strings.Contains(normalized, "模型")
	hasAction := strings.Contains(normalized, "下载") || strings.Contains(normalized, "安装") || strings.Contains(normalized, "download") || strings.Contains(normalized, "install")
	return hasModel && hasAction
}

func modelTestText(text string) bool {
	normalized := strings.ToLower(text)
	hasModel := strings.Contains(normalized, "locateanything")
	hasDataset := strings.Contains(normalized, "shanghaitech") || strings.Contains(normalized, "上海")
	hasAction := strings.Contains(normalized, "测试") || strings.Contains(normalized, "验证") || strings.Contains(normalized, "dry-run") || strings.Contains(normalized, "dry run") || strings.Contains(normalized, "smoke")
	return hasModel && hasDataset && hasAction
}

func dataIntakeText(text string) bool {
	if strings.HasPrefix(strings.TrimSpace(text), "/") {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(text, " ", ""))
	hasData := strings.Contains(normalized, "数据") ||
		strings.Contains(normalized, "dataset") ||
		strings.Contains(normalized, "data_lake") ||
		strings.Contains(normalized, "datalake") ||
		strings.Contains(normalized, "shanghaitech") ||
		strings.Contains(normalized, "上海")
	hasIntake := strings.Contains(normalized, "接入") ||
		strings.Contains(normalized, "入湖") ||
		strings.Contains(normalized, "注册") ||
		strings.Contains(normalized, "导入") ||
		strings.Contains(normalized, "上传") ||
		strings.Contains(normalized, "规划") ||
		strings.Contains(normalized, "manifest") ||
		strings.Contains(normalized, "本地文件夹") ||
		strings.Contains(normalized, "folder")
	if hasData && hasIntake {
		return true
	}
	if strings.Contains(normalized, "shanghaitech") && !strings.Contains(normalized, "locateanything") {
		return true
	}
	return (strings.Contains(normalized, "manifest") || strings.Contains(normalized, "zip")) &&
		(strings.Contains(normalized, "接入") || strings.Contains(normalized, "注册") || strings.Contains(normalized, "入湖"))
}

func inferredDatasetID(text string) string {
	normalized := strings.ToLower(text)
	if strings.Contains(normalized, "shanghaitech") || strings.Contains(normalized, "上海") {
		return "shanghaitech-original"
	}
	return ""
}
