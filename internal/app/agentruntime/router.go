package agentruntime

import (
	"os"
	"strings"
)

type PlanningRouteMode string

const (
	RouteLocalControl  PlanningRouteMode = "local_control"
	RouteLocalSemantic PlanningRouteMode = "local_semantic"
	RouteExternalPlan  PlanningRouteMode = "external_planner"
)

type PlanningRoute struct {
	Mode   PlanningRouteMode `json:"mode"`
	Reason string            `json:"reason"`
}

type RuntimeRouter struct{}

func NewRuntimeRouter() *RuntimeRouter {
	return &RuntimeRouter{}
}

func (r *RuntimeRouter) Select(req PlanRequest) PlanningRoute {
	if isControlIntent(req.Intent) {
		return PlanningRoute{Mode: RouteLocalControl, Reason: "deterministic command handled by Go control plane"}
	}
	if shouldUseLocalSemanticPlan(req.Intent) {
		return PlanningRoute{Mode: RouteLocalSemantic, Reason: "high-confidence semantic route handled before Mimo planner"}
	}
	return PlanningRoute{Mode: RouteExternalPlan, Reason: "requires LLM planner or parameter refinement"}
}

func isControlIntent(intent Intent) bool {
	switch intent.Kind {
	case IntentHealthCheck, IntentIdentifyActor, IntentRuntimeStatus, IntentListRuns, IntentSubmitDryRun, IntentRuntimeAbout, IntentVerifyHFJob, IntentTrainingDryRun, IntentEvaluationDryRun, IntentDeploymentDryRun:
		return true
	case IntentUnknown:
		return intent.Command == "/bot-help"
	default:
		return false
	}
}

func shouldUseLocalSemanticPlan(intent Intent) bool {
	if isFalseEnv(os.Getenv("AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH")) {
		return false
	}
	switch intent.Kind {
	case IntentModelInstall, IntentModelTest:
		return true
	case IntentDataIntake:
		return strings.EqualFold(intent.Metadata["local_semantic_fast_path"], "true")
	default:
		return false
	}
}
