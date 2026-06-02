package agentruntime

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/app/toolapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type SessionRunner interface {
	Run(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error)
}

type PlannerPort interface {
	Plan(ctx context.Context, req PlanRequest) (PlanResult, error)
}

type StreamingPlannerPort interface {
	PlanStream(ctx context.Context, req PlanRequest, emit func(RuntimeStreamEvent)) (PlanResult, error)
}

type ToolExecutorPort interface {
	Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error)
}

type StreamingToolExecutorPort interface {
	ExecuteStream(ctx context.Context, req ToolExecutionRequest, emit func(RuntimeStreamEvent)) (ToolExecutionResult, error)
}

type RuntimeStreamEvent struct {
	Type          string         `json:"type"`
	Delta         string         `json:"delta,omitempty"`
	Text          string         `json:"text,omitempty"`
	Status        string         `json:"status,omitempty"`
	Message       string         `json:"message,omitempty"`
	Intent        string         `json:"intent,omitempty"`
	AgentID       string         `json:"agent_id,omitempty"`
	ToolIDs       []string       `json:"tool_ids,omitempty"`
	ToolID        string         `json:"tool_id,omitempty"`
	Session       string         `json:"session,omitempty"`
	ElapsedMS     int64          `json:"elapsed_ms,omitempty"`
	ErrorEnvelope *ErrorEnvelope `json:"error_envelope,omitempty"`
}

type PlanRequest struct {
	Message    channel.InboundMessage `json:"message"`
	Session    SessionContext         `json:"session"`
	Intent     Intent                 `json:"intent"`
	Delegation DelegationDecision     `json:"delegation"`
}

type PlanResult struct {
	Intent     Intent             `json:"intent"`
	Delegation DelegationDecision `json:"delegation"`
	ReplyText  string             `json:"reply_text,omitempty"`
	ToolCalls  []ToolCall         `json:"tool_calls,omitempty"`
	Status     string             `json:"status,omitempty"`
}

type ToolCall = toolapp.ToolCall

type ToolExecutionRequest struct {
	Message    channel.InboundMessage `json:"message"`
	Session    SessionContext         `json:"session"`
	Intent     Intent                 `json:"intent"`
	Delegation DelegationDecision     `json:"delegation"`
	ToolCalls  []ToolCall             `json:"tool_calls"`
}

type ToolExecutionResult = toolapp.ExecutionResult
