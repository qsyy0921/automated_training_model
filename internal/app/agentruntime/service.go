package agentruntime

import (
	"context"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

const defaultWorkflowID = "data-to-deployment-lifecycle"

type AgentControlPlane interface {
	SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error)
	ListRuns(ctx context.Context) ([]agent.WorkflowRun, error)
}

type Service struct {
	runner SessionRunner
}

func NewService(agents AgentControlPlane) *Service {
	now := time.Now
	return NewServiceWithPorts(PlannerFromEnv(), NewGoToolExecutor(agents, now), now)
}

func NewServiceWithPorts(planner PlannerPort, tools ToolExecutorPort, now func() time.Time) *Service {
	return NewServiceWithRunner(NewDefaultSessionRunner(planner, tools, now))
}

func NewServiceWithRunner(runner SessionRunner) *Service {
	return &Service{runner: runner}
}

func (s *Service) HandleChannelMessage(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error) {
	return s.runner.Run(ctx, msg)
}

func (s *Service) Snapshot(limit int) RuntimeSnapshot {
	if runner, ok := s.runner.(interface{ Snapshot(int) RuntimeSnapshot }); ok {
		return runner.Snapshot(limit)
	}
	return RuntimeSnapshot{}
}

func (s *Service) ListSessions() []SessionState {
	if runner, ok := s.runner.(interface{ ListSessions() []SessionState }); ok {
		return runner.ListSessions()
	}
	return nil
}

func (s *Service) ListTraces(limit int) []TraceEvent {
	if runner, ok := s.runner.(interface{ ListTraces(int) []TraceEvent }); ok {
		return runner.ListTraces(limit)
	}
	return nil
}
