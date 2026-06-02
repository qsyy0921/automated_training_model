package toolapp

import (
	"context"
	"fmt"
	"strings"
)

type ExecutionResult struct {
	ReplyText string            `json:"reply_text,omitempty"`
	Status    string            `json:"status,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ProgressEvent struct {
	Type     string            `json:"type"`
	ToolID   string            `json:"tool_id,omitempty"`
	Status   string            `json:"status,omitempty"`
	Message  string            `json:"message,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Handler[Req any] func(context.Context, Req, ToolCall) (ExecutionResult, error)

type PolicyProvider func() PreflightPolicy

type Runner[Req any] struct {
	catalog        Catalog
	policyProvider PolicyProvider
	handlers       map[string]Handler[Req]
}

func NewRunner[Req any](catalog Catalog, policyProvider PolicyProvider) *Runner[Req] {
	if policyProvider == nil {
		policyProvider = func() PreflightPolicy { return PreflightPolicy{} }
	}
	return &Runner[Req]{
		catalog:        catalog,
		policyProvider: policyProvider,
		handlers:       map[string]Handler[Req]{},
	}
}

func (r *Runner[Req]) Register(toolID string, handler Handler[Req]) {
	if strings.TrimSpace(toolID) == "" || handler == nil {
		return
	}
	r.handlers[toolID] = handler
}

func (r *Runner[Req]) Execute(ctx context.Context, req Req, calls []ToolCall) (ExecutionResult, error) {
	return r.ExecuteStream(ctx, req, calls, nil)
}

func (r *Runner[Req]) ExecuteStream(ctx context.Context, req Req, calls []ToolCall, emit func(ProgressEvent)) (ExecutionResult, error) {
	if len(calls) == 0 {
		return ExecutionResult{}, nil
	}
	safeEmit := func(event ProgressEvent) {
		if emit != nil {
			emit(event)
		}
	}
	results := make([]string, 0, len(calls))
	status := "ok"
	metadata := map[string]string{}
	for _, call := range calls {
		safeEmit(ProgressEvent{Type: "preflight_start", ToolID: call.ToolID, Message: "checking tool schema and policy"})
		preflight := Preflight(r.catalog, r.policyProvider(), call)
		if !preflight.Allowed {
			safeEmit(ProgressEvent{Type: "preflight_blocked", ToolID: call.ToolID, Status: preflight.Status, Message: preflight.Message, Metadata: preflight.Metadata})
			return ExecutionResult{
				ReplyText: preflight.Message,
				Status:    preflight.Status,
				Metadata:  preflight.Metadata,
			}, nil
		}
		safeEmit(ProgressEvent{Type: "preflight_ok", ToolID: call.ToolID, Status: "ok", Message: "tool preflight passed"})
		handler, ok := r.handlers[call.ToolID]
		if !ok {
			metadata := map[string]string{
				"tool_id": call.ToolID,
				"reason":  "handler_not_registered",
			}
			safeEmit(ProgressEvent{Type: "tool_unsupported", ToolID: call.ToolID, Status: "unsupported_tool", Message: fmt.Sprintf("tool %s has no registered handler", call.ToolID), Metadata: metadata})
			return ExecutionResult{
				ReplyText: fmt.Sprintf("工具 %s 尚未接入。", call.ToolID),
				Status:    "unsupported_tool",
				Metadata:  metadata,
			}, nil
		}
		safeEmit(ProgressEvent{Type: "tool_start", ToolID: call.ToolID, Status: "running", Message: "running tool handler"})
		result, err := handler(ctx, req, call)
		if err != nil {
			safeEmit(ProgressEvent{Type: "tool_error", ToolID: call.ToolID, Status: "failed", Message: err.Error()})
			return ExecutionResult{}, err
		}
		safeEmit(ProgressEvent{Type: "tool_done", ToolID: call.ToolID, Status: result.Status, Message: "tool handler completed", Metadata: result.Metadata})
		if result.Status != "" {
			status = result.Status
		}
		if result.ReplyText != "" {
			results = append(results, result.ReplyText)
		}
		for key, value := range result.Metadata {
			metadata[key] = value
		}
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return ExecutionResult{ReplyText: strings.Join(results, "\n"), Status: status, Metadata: metadata}, nil
}
