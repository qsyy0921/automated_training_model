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

func TestPythonPlannerConfigUsesWorkerByDefault(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_PYTHON_WORKER", "")

	cfg := PythonPlannerConfigFromEnv()
	if !cfg.Worker {
		t.Fatal("PythonPlannerConfigFromEnv() did not enable worker transport by default")
	}
}

func TestPythonPlannerConfigCanDisableWorker(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_PYTHON_WORKER", "false")

	cfg := PythonPlannerConfigFromEnv()
	if cfg.Worker {
		t.Fatal("PythonPlannerConfigFromEnv() did not honor AGENT_RUNTIME_PYTHON_WORKER=false")
	}
	if got := plannerStatusFromEnv().Transport; got != "python-spawn" {
		t.Fatalf("planner transport = %q, want python-spawn", got)
	}
}
