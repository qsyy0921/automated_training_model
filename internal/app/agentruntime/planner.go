package agentruntime

import (
	"context"
	"fmt"
	"strings"
)

type RulePlanner struct{}

func NewRulePlanner() *RulePlanner {
	return &RulePlanner{}
}

func (p *RulePlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	result := PlanResult{Intent: req.Intent, Delegation: req.Delegation, Status: "planned"}
	switch req.Intent.Kind {
	case IntentHealthCheck, IntentIdentifyActor, IntentRuntimeStatus, IntentListRuns, IntentSubmitDryRun, IntentDataIntake:
		return p.toolPlan(req), nil
	case IntentRuntimeAbout:
		result.ReplyText = strings.Join([]string{
			"我是 automated_training_model 的本地 Agent Runtime，不是单纯的模型聊天窗口。",
			"当前入口包括 CLI、Web、桌面端脚手架和 QQ/NapCat；Go Gateway 负责连接、权限、状态和审计，Python/Mimo 负责语义规划、多模态理解和工具计划。",
			"确定性的控制意图会在本地快速处理；下载模型、数据接入、训练、评估和部署会进入 planner-agent 或对应 sub-agent 的受控工具流程。",
		}, "\n")
		return result, nil
	case IntentModelInstall:
		return p.locateAnythingInstallPlan(req), nil
	case IntentModelTest:
		return p.locateAnythingTestPlan(req), nil
	case IntentChat:
		result.ReplyText = fmt.Sprintf("已收到。意图会先交给 %s 做规划；当前最小运行时已支持 /bot-status、/bot-runs 和 /bot-run dry。", req.Delegation.AgentID)
		return result, nil
	default:
		if req.Intent.Command == "/bot-help" {
			result.ReplyText = strings.Join([]string{
				"可用命令：",
				"/bot-ping",
				"/bot-me",
				"/bot-status",
				"/bot-runs",
				"/bot-run dry [dataset_id]",
				"普通文本 -> planner-agent",
				"图片/附件 -> vision-agent 或 data-intake-agent",
			}, "\n")
			return result, nil
		}
		result.ReplyText = strings.Join([]string{
			"未知命令或暂不支持的意图。",
			"发送 /bot-help 查看可用命令。",
		}, "\n")
		return result, nil
	}
}

func (p *RulePlanner) locateAnythingInstallPlan(req PlanRequest) PlanResult {
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为 HuggingFace 模型安装请求，使用本地高置信度路由生成受控下载计划。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "model.download_hf",
			SkillID: valueOrString(req.Intent.SkillID, "huggingface-model-downloader"),
			Params: map[string]string{
				"repo_id":   "nvidia/LocateAnything-3B",
				"local_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
				"manifest":  "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
			},
		}},
	}
}

func (p *RulePlanner) locateAnythingTestPlan(req PlanRequest) PlanResult {
	dataRoot := "data_lake/raw/datasets/shanghaitech/original"
	if strings.Contains(strings.ToLower(req.Message.Text), "f:\\") {
		dataRoot = "F:\\automated_training_model\\data_lake\\raw\\datasets\\shanghaitech\\original"
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为 LocateAnything-3B + ShanghaiTech 测试请求，使用本地高置信度路由生成校验、smoke 和 dry-run 计划。",
		ToolCalls: []ToolCall{
			{
				ID:      "call-1",
				ToolID:  "model.verify_hf",
				SkillID: "huggingface-model-downloader",
				Params: map[string]string{
					"repo_id":     "nvidia/LocateAnything-3B",
					"local_dir":   "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
					"manifest":    "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
					"verify_only": "true",
				},
			},
			{
				ID:      "call-2",
				ToolID:  "model.smoke_locateanything",
				SkillID: "model-validation",
				Params: map[string]string{
					"model_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
					"data_root": dataRoot,
					"output":    "data_lake/catalog/models/nvidia_LocateAnything-3B.smoke.json",
				},
			},
			{
				ID:      "call-3",
				ToolID:  "workflow.submit_run",
				SkillID: "data-to-deployment-lifecycle",
				Params: map[string]string{
					"workflow_id":   "data-to-deployment-lifecycle",
					"dataset_id":    valueOrString(req.Intent.DatasetID, "shanghaitech-original"),
					"dry_run":       "true",
					"model_repo_id": "nvidia/LocateAnything-3B",
					"data_root":     dataRoot,
				},
			},
		},
	}
}

func (p *RulePlanner) toolPlan(req PlanRequest) PlanResult {
	toolID := req.Delegation.ToolID
	if toolID == "" {
		toolID = req.Intent.ToolID
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned",
		ToolCalls: []ToolCall{{
			ID:        "call-1",
			ToolID:    toolID,
			SkillID:   req.Intent.SkillID,
			MCPServer: req.Intent.MCPServer,
			Params:    toolParams(req),
		}},
	}
}

func toolParams(req PlanRequest) map[string]string {
	params := map[string]string{
		"source":      string(req.Message.Channel),
		"account_id":  req.Message.AccountID,
		"peer_kind":   string(req.Message.Peer.Kind),
		"peer_id":     req.Message.Peer.ID,
		"sender_id":   req.Message.SenderID,
		"session_key": req.Session.Key,
		"agent_id":    req.Session.AgentID,
	}
	if req.Intent.DatasetID != "" {
		params["dataset_id"] = req.Intent.DatasetID
	}
	return params
}

func valueOrString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
