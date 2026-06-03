package agentruntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/app/runtimeworkflow"
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
			"当前入口包括 CLI、Web、桌面端 runtime 面板和 QQ/NapCat；Go Gateway 负责连接、权限、状态和审计，Python/Mimo 负责语义规划、多模态理解和工具计划。",
			"确定性的控制意图会在本地快速处理；下载模型、数据接入、训练、评估和部署会进入 planner-agent 或对应 sub-agent 的受控工具流程。",
		}, "\n")
		return result, nil
	case IntentModelInstall:
		return p.locateAnythingInstallPlan(req), nil
	case IntentModelTest:
		return p.locateAnythingTestPlan(req), nil
	case IntentVerifyHFJob:
		return p.locateAnythingVerifyJobPlan(req), nil
	case IntentTrainingDryRun:
		return p.trainingDryRunPlan(req), nil
	case IntentTrainingRun:
		return p.trainingRunPlan(req), nil
	case IntentEvaluationDryRun:
		return p.evaluationDryRunPlan(req), nil
	case IntentEvaluationRun:
		return p.evaluationRunPlan(req), nil
	case IntentDeploymentDryRun:
		return p.deploymentDryRunPlan(req), nil
	case IntentDeploymentRun:
		return p.deploymentRunPlan(req), nil
	case IntentChat:
		result.ReplyText = fmt.Sprintf("已收到。意图会先交给 %s 做规划；当前最小运行时已支持 /bot-status、/bot-runs、/bot-train-dry|run、/bot-eval-dry|run、/bot-deploy-dry|run。", req.Delegation.AgentID)
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
				"/bot-verify-hf-job [repo_id]",
				"/bot-train-dry <dataset_id> [target_task] [model_family]",
				"/bot-train-run <dataset_id> [target_task] [model_family]",
				"/bot-eval-dry <dataset_id> <model_id> [split]",
				"/bot-eval-run <dataset_id> <model_id> [split]",
				"/bot-deploy-dry <model_id> <target> [runtime] [replicas]",
				"/bot-deploy-run <model_id> <target> [runtime] [replicas]",
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
					"workflow_id":   runtimeworkflow.DefaultWorkflowID,
					"dataset_id":    valueOrString(req.Intent.DatasetID, "shanghaitech-original"),
					"dry_run":       "true",
					"model_repo_id": "nvidia/LocateAnything-3B",
					"data_root":     dataRoot,
				},
			},
		},
	}
}

func (p *RulePlanner) locateAnythingVerifyJobPlan(req PlanRequest) PlanResult {
	repoID := "nvidia/LocateAnything-3B"
	if len(req.Intent.Args) >= 1 && strings.TrimSpace(req.Intent.Args[0]) != "" {
		repoID = strings.TrimSpace(req.Intent.Args[0])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为 HuggingFace verify worker 请求，使用本地确定性命令生成后台校验任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "model.verify_hf",
			SkillID: "huggingface-model-downloader",
			Params: map[string]string{
				"repo_id":     repoID,
				"local_dir":   filepath.Join("data_lake", "models", "artifacts", "huggingface", strings.ReplaceAll(repoID, "/", string(filepath.Separator))),
				"manifest":    filepath.Join("data_lake", "catalog", "models", strings.ReplaceAll(repoID, "/", "_")+".download.json"),
				"verify_only": "true",
				"job":         "true",
			},
		}},
	}
}

func (p *RulePlanner) trainingDryRunPlan(req PlanRequest) PlanResult {
	datasetID := valueOrString(req.Intent.DatasetID, "shanghaitech-original")
	targetTask := "detection"
	modelFamily := "yolo11n"
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		targetTask = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		modelFamily = strings.TrimSpace(req.Intent.Args[2])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为训练 dry-run 请求，使用本地确定性命令生成后台训练 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "training.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"dataset_id":   datasetID,
				"target_task":  targetTask,
				"model_family": modelFamily,
				"dry_run":      "true",
			},
		}},
	}
}

func (p *RulePlanner) trainingRunPlan(req PlanRequest) PlanResult {
	datasetID := valueOrString(req.Intent.DatasetID, "shanghaitech-original")
	targetTask := "detection"
	modelFamily := "yolo11n"
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		targetTask = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		modelFamily = strings.TrimSpace(req.Intent.Args[2])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为训练执行请求，使用本地确定性命令生成后台训练 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "training.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"dataset_id":       datasetID,
				"target_task":      targetTask,
				"model_family":     modelFamily,
				"dry_run":          "false",
				"execution_recipe": "default",
			},
		}},
	}
}

func (p *RulePlanner) evaluationDryRunPlan(req PlanRequest) PlanResult {
	datasetID := valueOrString(req.Intent.DatasetID, "shanghaitech-original")
	modelID := "candidate-model"
	split := "validation"
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		modelID = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		split = strings.TrimSpace(req.Intent.Args[2])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为评估 dry-run 请求，使用本地确定性命令生成后台评估 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "evaluation.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"dataset_id": datasetID,
				"model_id":   modelID,
				"split":      split,
				"dry_run":    "true",
			},
		}},
	}
}

func (p *RulePlanner) evaluationRunPlan(req PlanRequest) PlanResult {
	datasetID := valueOrString(req.Intent.DatasetID, "shanghaitech-original")
	modelID := "candidate-model"
	split := "validation"
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		modelID = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		split = strings.TrimSpace(req.Intent.Args[2])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为评估执行请求，使用本地确定性命令生成后台评估 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "evaluation.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"dataset_id":       datasetID,
				"model_id":         modelID,
				"split":            split,
				"dry_run":          "false",
				"execution_recipe": "default",
			},
		}},
	}
}

func (p *RulePlanner) deploymentDryRunPlan(req PlanRequest) PlanResult {
	modelID := "candidate-model"
	target := "local-dry-run"
	runtime := "python-worker"
	replicas := "1"
	if len(req.Intent.Args) >= 1 && strings.TrimSpace(req.Intent.Args[0]) != "" {
		modelID = strings.TrimSpace(req.Intent.Args[0])
	}
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		target = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		runtime = strings.TrimSpace(req.Intent.Args[2])
	}
	if len(req.Intent.Args) >= 4 && strings.TrimSpace(req.Intent.Args[3]) != "" {
		replicas = strings.TrimSpace(req.Intent.Args[3])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为部署 dry-run 请求，使用本地确定性命令生成后台部署 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "deployment.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"model_id": modelID,
				"target":   target,
				"runtime":  runtime,
				"replicas": replicas,
				"dry_run":  "true",
			},
		}},
	}
}

func (p *RulePlanner) deploymentRunPlan(req PlanRequest) PlanResult {
	modelID := "candidate-model"
	target := "local-dry-run"
	runtime := "python-worker"
	replicas := "1"
	if len(req.Intent.Args) >= 1 && strings.TrimSpace(req.Intent.Args[0]) != "" {
		modelID = strings.TrimSpace(req.Intent.Args[0])
	}
	if len(req.Intent.Args) >= 2 && strings.TrimSpace(req.Intent.Args[1]) != "" {
		target = strings.TrimSpace(req.Intent.Args[1])
	}
	if len(req.Intent.Args) >= 3 && strings.TrimSpace(req.Intent.Args[2]) != "" {
		runtime = strings.TrimSpace(req.Intent.Args[2])
	}
	if len(req.Intent.Args) >= 4 && strings.TrimSpace(req.Intent.Args[3]) != "" {
		replicas = strings.TrimSpace(req.Intent.Args[3])
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned_with_guard",
		ReplyText:  "已识别为部署执行请求，使用本地确定性命令生成后台部署 worker 任务。",
		ToolCalls: []ToolCall{{
			ID:      "call-1",
			ToolID:  "deployment.run",
			SkillID: "data-to-deployment-lifecycle",
			Params: map[string]string{
				"model_id":         modelID,
				"target":           target,
				"runtime":          runtime,
				"replicas":         replicas,
				"dry_run":          "false",
				"execution_recipe": "default",
			},
		}},
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
