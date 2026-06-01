package agentruntime

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type SessionRunner interface {
	Run(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error)
}

type PlannerPort interface {
	Plan(ctx context.Context, req PlanRequest) (PlanResult, error)
}

type ToolExecutorPort interface {
	Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error)
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

type ToolCall struct {
	ID               string            `json:"id"`
	ToolID           string            `json:"tool_id"`
	SkillID          string            `json:"skill_id,omitempty"`
	MCPServer        string            `json:"mcp_server,omitempty"`
	Params           map[string]string `json:"params,omitempty"`
	RequiresApproval bool              `json:"requires_approval,omitempty"`
}

type ToolExecutionRequest struct {
	Message    channel.InboundMessage `json:"message"`
	Session    SessionContext         `json:"session"`
	Intent     Intent                 `json:"intent"`
	Delegation DelegationDecision     `json:"delegation"`
	ToolCalls  []ToolCall             `json:"tool_calls"`
}

type ToolExecutionResult struct {
	ReplyText string            `json:"reply_text,omitempty"`
	Status    string            `json:"status,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
