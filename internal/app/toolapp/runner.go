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
	if len(calls) == 0 {
		return ExecutionResult{}, nil
	}
	results := make([]string, 0, len(calls))
	status := "ok"
	metadata := map[string]string{}
	for _, call := range calls {
		preflight := Preflight(r.catalog, r.policyProvider(), call)
		if !preflight.Allowed {
			return ExecutionResult{
				ReplyText: preflight.Message,
				Status:    preflight.Status,
				Metadata:  preflight.Metadata,
			}, nil
		}
		handler, ok := r.handlers[call.ToolID]
		if !ok {
			return ExecutionResult{
				ReplyText: fmt.Sprintf("工具 %s 尚未接入。", call.ToolID),
				Status:    "unsupported_tool",
				Metadata: map[string]string{
					"tool_id": call.ToolID,
					"reason":  "handler_not_registered",
				},
			}, nil
		}
		result, err := handler(ctx, req, call)
		if err != nil {
			return ExecutionResult{}, err
		}
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
