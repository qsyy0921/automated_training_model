package agentruntime

import (
	"context"
	"errors"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/app/runtimeworkflow"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

var ErrUnsupportedModelJobAction = errors.New("model job action is not supported by this runtime")
var ErrUnsupportedIntakeWorkflowAction = errors.New("intake workflow action is not supported by this runtime")

type AgentControlPlane = runtimeworkflow.ControlPlane

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
			envelope := ErrorEnvelopeFromStatus("failed", "agent-runtime", err)
			emit(RuntimeStreamEvent{Type: "error", Message: envelope.Message, Status: "failed", ErrorEnvelope: &envelope})
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

func (s *Service) ListIntakeWorkflows(ctx context.Context, limit int) ([]intakeapp.IntakeWorkflow, error) {
	if runner, ok := s.runner.(interface {
		ListIntakeWorkflows(context.Context, int) ([]intakeapp.IntakeWorkflow, error)
	}); ok {
		return runner.ListIntakeWorkflows(ctx, limit)
	}
	return nil, ErrUnsupportedIntakeWorkflowAction
}

func (s *Service) GetIntakeWorkflow(ctx context.Context, id string) (intakeapp.IntakeWorkflow, bool, error) {
	if runner, ok := s.runner.(interface {
		GetIntakeWorkflow(context.Context, string) (intakeapp.IntakeWorkflow, bool, error)
	}); ok {
		return runner.GetIntakeWorkflow(ctx, id)
	}
	return intakeapp.IntakeWorkflow{}, false, ErrUnsupportedIntakeWorkflowAction
}

func (s *Service) ApproveIntakeWorkflow(ctx context.Context, id string, by string, note string) (intakeapp.IntakeWorkflow, error) {
	if runner, ok := s.runner.(interface {
		ApproveIntakeWorkflow(context.Context, string, string, string) (intakeapp.IntakeWorkflow, error)
	}); ok {
		return runner.ApproveIntakeWorkflow(ctx, id, by, note)
	}
	return intakeapp.IntakeWorkflow{}, ErrUnsupportedIntakeWorkflowAction
}

func (s *Service) RegisterIntakeWorkflow(ctx context.Context, id string, by string) (intakeapp.IntakeWorkflow, error) {
	if runner, ok := s.runner.(interface {
		RegisterIntakeWorkflow(context.Context, string, string) (intakeapp.IntakeWorkflow, error)
	}); ok {
		return runner.RegisterIntakeWorkflow(ctx, id, by)
	}
	return intakeapp.IntakeWorkflow{}, ErrUnsupportedIntakeWorkflowAction
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
