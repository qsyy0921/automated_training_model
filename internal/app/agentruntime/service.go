package agentruntime

import (
	"context"
	"errors"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

const defaultWorkflowID = "data-to-deployment-lifecycle"

var ErrUnsupportedModelJobAction = errors.New("model job action is not supported by this runtime")

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

func NewServiceWithStore(agents AgentControlPlane, store RuntimeStore) *Service {
	now := time.Now
	return NewServiceWithRunner(NewDefaultSessionRunnerWithStore(PlannerFromEnv(), NewGoToolExecutor(agents, now), store, now))
}

func NewServiceWithStores(agents AgentControlPlane, runtimeStore RuntimeStore, modelJobs ModelJobStore) *Service {
	now := time.Now
	return NewServiceWithRunner(NewDefaultSessionRunnerWithStore(PlannerFromEnv(), NewGoToolExecutorWithModelJobs(agents, now, modelJobs), runtimeStore, now))
}

func NewServiceWithRuntimeStores(agents AgentControlPlane, runtimeStore RuntimeStore, modelJobs ModelJobStore, intakeRepo intakeapp.Repository) *Service {
	now := time.Now
	return NewServiceWithRunner(NewDefaultSessionRunnerWithStore(PlannerFromEnv(), NewGoToolExecutorWithStores(agents, now, modelJobs, intakeRepo), runtimeStore, now))
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

func (s *Service) HandleChannelMessageStream(ctx context.Context, msg channel.InboundMessage, emit func(RuntimeStreamEvent)) (channel.OutboundMessage, error) {
	if runner, ok := s.runner.(interface {
		RunStream(context.Context, channel.InboundMessage, func(RuntimeStreamEvent)) (channel.OutboundMessage, error)
	}); ok {
		return runner.RunStream(ctx, msg, emit)
	}
	reply, err := s.runner.Run(ctx, msg)
	if emit != nil {
		if err != nil {
			emit(RuntimeStreamEvent{Type: "error", Message: err.Error(), Status: "failed"})
		} else {
			emit(RuntimeStreamEvent{Type: "final", Text: reply.Text, Status: "ok"})
		}
	}
	return reply, err
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

func (s *Service) ListModelJobs(limit int) []ModelJob {
	if runner, ok := s.runner.(interface{ ListModelJobs(int) []ModelJob }); ok {
		return runner.ListModelJobs(limit)
	}
	return nil
}

func (s *Service) GetModelJob(id string) (ModelJob, bool) {
	if runner, ok := s.runner.(interface{ GetModelJob(string) (ModelJob, bool) }); ok {
		return runner.GetModelJob(id)
	}
	return ModelJob{}, false
}

func (s *Service) CancelModelJob(id string) (ModelJob, error) {
	if runner, ok := s.runner.(interface {
		CancelModelJob(string) (ModelJob, error)
	}); ok {
		return runner.CancelModelJob(id)
	}
	return ModelJob{}, ErrUnsupportedModelJobAction
}

func (s *Service) ResumeModelJob(id string) (ModelJob, error) {
	if runner, ok := s.runner.(interface {
		ResumeModelJob(string) (ModelJob, error)
	}); ok {
		return runner.ResumeModelJob(id)
	}
	return ModelJob{}, ErrUnsupportedModelJobAction
}
