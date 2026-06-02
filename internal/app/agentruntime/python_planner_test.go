package agentruntime

import "testing"

func TestPlannerFromEnvUsesPythonWhenMimoEnabled(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_PLANNER", "")
	t.Setenv("AGENT_RUNTIME_USE_MIMO", "true")

	if _, ok := PlannerFromEnv().(*PythonPlanner); !ok {
		t.Fatal("PlannerFromEnv() did not select PythonPlanner when AGENT_RUNTIME_USE_MIMO=true")
	}
}

func TestPlannerFromEnvRuleOverrideWins(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_PLANNER", "rule")
	t.Setenv("AGENT_RUNTIME_USE_MIMO", "true")

	if _, ok := PlannerFromEnv().(*RulePlanner); !ok {
		t.Fatal("PlannerFromEnv() did not honor AGENT_RUNTIME_PLANNER=rule")
	}
}
