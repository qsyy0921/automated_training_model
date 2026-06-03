package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type SessionContext struct {
	Key       string           `json:"key"`
	AgentID   string           `json:"agent_id"`
	Channel   channel.Kind     `json:"channel"`
	AccountID string           `json:"account_id"`
	Peer      channel.Peer     `json:"peer"`
	SenderID  string           `json:"sender_id"`
	PeerKind  channel.PeerKind `json:"peer_kind"`
	PeerID    string           `json:"peer_id"`
}

type DefaultSessionRunner struct {
	planner PlannerPort
	tools   ToolExecutorPort
	store   RuntimeStore
	router  *RuntimeRouter
	now     func() time.Time
}

func NewDefaultSessionRunner(planner PlannerPort, tools ToolExecutorPort, now func() time.Time) *DefaultSessionRunner {
	if now == nil {
		now = time.Now
	}
	return NewDefaultSessionRunnerWithStore(planner, tools, NewInMemoryRuntimeStore(now()), now)
}

func NewDefaultSessionRunnerWithStore(planner PlannerPort, tools ToolExecutorPort, store RuntimeStore, now func() time.Time) *DefaultSessionRunner {
	if now == nil {
		now = time.Now
	}
	if store == nil {
		store = NewInMemoryRuntimeStore(now())
	}
	return &DefaultSessionRunner{planner: planner, tools: tools, store: store, router: NewRuntimeRouter(), now: now}
}

func (r *DefaultSessionRunner) Run(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error) {
	intent := ClassifyIntent(msg)
	delegation := DecideSubAgent(intent, msg)
	session := BuildSessionContext(msg, delegation)
	reply := channel.OutboundMessage{
		Channel:   msg.Channel,
		AccountID: msg.AccountID,
		Peer:      msg.Peer,
		ReplyToID: msg.ID,
	}

	if strings.TrimSpace(msg.Text) == "" && len(msg.Attachments) == 0 {
		reply.Text = "已收到空消息。"
		r.record(session, msg, intent, nil, "ok", reply.Text, "", nil)
		return reply, nil
	}

	planReq := PlanRequest{
		Message:    msg,
		Session:    session,
		Intent:     intent,
		Delegation: delegation,
	}
	plan, err := r.plan(ctx, planReq)
	if err != nil {
		r.record(session, msg, intent, nil, "planning_failed", "", err.Error(), nil)
		return channel.OutboundMessage{}, err
	}
	if len(plan.ToolCalls) == 0 {
		reply.Text = plan.ReplyText
		r.record(session, msg, plan.Intent, nil, plan.Status, reply.Text, "", nil)
		return reply, nil
	}

	result, err := r.tools.Execute(ctx, ToolExecutionRequest{
		Message:    msg,
		Session:    session,
		Intent:     plan.Intent,
		Delegation: plan.Delegation,
		ToolCalls:  plan.ToolCalls,
	})
	if err != nil {
		r.record(session, msg, plan.Intent, plan.ToolCalls, "tool_failed", "", err.Error(), nil)
		return channel.OutboundMessage{}, err
	}
	reply.Text = result.ReplyText
	r.record(session, msg, plan.Intent, plan.ToolCalls, result.Status, reply.Text, "", result.Metadata)
	return reply, nil
}

func (r *DefaultSessionRunner) RunStream(ctx context.Context, msg channel.InboundMessage, emit func(RuntimeStreamEvent)) (channel.OutboundMessage, error) {
	started := r.now()
	intent := ClassifyIntent(msg)
	delegation := DecideSubAgent(intent, msg)
	session := BuildSessionContext(msg, delegation)
	reply := channel.OutboundMessage{
		Channel:   msg.Channel,
		AccountID: msg.AccountID,
		Peer:      msg.Peer,
		ReplyToID: msg.ID,
	}
	safeEmit := func(event RuntimeStreamEvent) {
		if emit != nil {
			if event.Session == "" {
				event.Session = session.Key
			}
			emit(event)
		}
	}
	safeEmit(RuntimeStreamEvent{Type: "status", Intent: string(intent.Kind), AgentID: session.AgentID, Message: "runtime accepted message"})

	if strings.TrimSpace(msg.Text) == "" && len(msg.Attachments) == 0 {
		reply.Text = "已收到空消息。"
		r.record(session, msg, intent, nil, "ok", reply.Text, "", nil)
		safeEmit(RuntimeStreamEvent{Type: "final", Text: reply.Text, Status: "ok", Intent: string(intent.Kind), AgentID: session.AgentID, ElapsedMS: r.now().Sub(started).Milliseconds()})
		return reply, nil
	}

	planReq := PlanRequest{Message: msg, Session: session, Intent: intent, Delegation: delegation}
	plan, err := r.planStream(ctx, planReq, safeEmit)
	if err != nil {
		envelope := ErrorEnvelopeFromStatus("planning_failed", session.AgentID, err)
		r.record(session, msg, intent, nil, "planning_failed", "", err.Error(), nil)
		safeEmit(RuntimeStreamEvent{Type: "error", Status: "planning_failed", Message: envelope.Message, Intent: string(intent.Kind), AgentID: session.AgentID, ElapsedMS: r.now().Sub(started).Milliseconds(), ErrorEnvelope: &envelope})
		return channel.OutboundMessage{}, err
	}
	if len(plan.ToolCalls) == 0 {
		reply.Text = plan.ReplyText
		r.record(session, msg, plan.Intent, nil, plan.Status, reply.Text, "", nil)
		safeEmit(RuntimeStreamEvent{Type: "final", Text: reply.Text, Status: plan.Status, Intent: string(plan.Intent.Kind), AgentID: session.AgentID, ElapsedMS: r.now().Sub(started).Milliseconds()})
		return reply, nil
	}

	safeEmit(RuntimeStreamEvent{Type: "tool_start", Intent: string(plan.Intent.Kind), AgentID: session.AgentID, ToolIDs: collectToolIDs(plan.ToolCalls), Message: "executing planned tools"})
	toolReq := ToolExecutionRequest{
		Message:    msg,
		Session:    session,
		Intent:     plan.Intent,
		Delegation: plan.Delegation,
		ToolCalls:  plan.ToolCalls,
	}
	var (
		result ToolExecutionResult
	)
	if streamingTools, ok := r.tools.(StreamingToolExecutorPort); ok {
		result, err = streamingTools.ExecuteStream(ctx, toolReq, func(event RuntimeStreamEvent) {
			if event.Intent == "" {
				event.Intent = string(plan.Intent.Kind)
			}
			if event.AgentID == "" {
				event.AgentID = session.AgentID
			}
			safeEmit(event)
		})
	} else {
		result, err = r.tools.Execute(ctx, toolReq)
	}
	if err != nil {
		envelope := ErrorEnvelopeFromStatus("tool_failed", session.AgentID, err)
		r.record(session, msg, plan.Intent, plan.ToolCalls, "tool_failed", "", err.Error(), nil)
		safeEmit(RuntimeStreamEvent{Type: "error", Status: "tool_failed", Message: envelope.Message, Intent: string(plan.Intent.Kind), AgentID: session.AgentID, ToolIDs: collectToolIDs(plan.ToolCalls), ElapsedMS: r.now().Sub(started).Milliseconds(), ErrorEnvelope: &envelope})
		return channel.OutboundMessage{}, err
	}
	reply.Text = result.ReplyText
	r.record(session, msg, plan.Intent, plan.ToolCalls, result.Status, reply.Text, "", result.Metadata)
	safeEmit(RuntimeStreamEvent{Type: "final", Text: reply.Text, Status: result.Status, Intent: string(plan.Intent.Kind), AgentID: session.AgentID, ToolIDs: collectToolIDs(plan.ToolCalls), ElapsedMS: r.now().Sub(started).Milliseconds()})
	return reply, nil
}

func (r *DefaultSessionRunner) plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	route := r.router.Select(req)
	if route.Mode == RouteLocalControl || route.Mode == RouteLocalSemantic {
		return NewRulePlanner().Plan(ctx, req)
	}
	plan, err := r.planner.Plan(ctx, req)
	if err != nil {
		return PlanResult{}, err
	}
	return r.enforceMandatoryPlan(ctx, req, plan)
}

func (r *DefaultSessionRunner) planStream(ctx context.Context, req PlanRequest, emit func(RuntimeStreamEvent)) (PlanResult, error) {
	route := r.router.Select(req)
	if route.Mode == RouteLocalControl || route.Mode == RouteLocalSemantic {
		if emit != nil {
			emit(RuntimeStreamEvent{Type: "status", Intent: string(req.Intent.Kind), AgentID: req.Session.AgentID, Message: string(route.Mode) + ": " + route.Reason})
		}
		return NewRulePlanner().Plan(ctx, req)
	}
	var plan PlanResult
	var err error
	if streamingPlanner, ok := r.planner.(StreamingPlannerPort); ok {
		plan, err = streamingPlanner.PlanStream(ctx, req, emit)
	} else {
		plan, err = r.planner.Plan(ctx, req)
	}
	if err != nil {
		return PlanResult{}, err
	}
	return r.enforceMandatoryPlan(ctx, req, plan)
}

func (r *DefaultSessionRunner) enforceMandatoryPlan(ctx context.Context, req PlanRequest, plan PlanResult) (PlanResult, error) {
	if req.Intent.Kind != IntentDataIntake {
		return plan, nil
	}
	plan.Delegation = req.Delegation
	if plan.Intent.Kind == "" {
		plan.Intent = req.Intent
	}
	requiredTool := "intake.plan"
	if req.Delegation.ToolID == "vlm.inspect" {
		requiredTool = "vlm.inspect"
	}
	if isExactToolPlan(plan.ToolCalls, requiredTool) {
		return plan, nil
	}
	return NewRulePlanner().Plan(ctx, req)
}

func (r *DefaultSessionRunner) Snapshot(limit int) RuntimeSnapshot {
	return r.store.Snapshot(limit)
}

func (r *DefaultSessionRunner) ListSessions() []SessionState {
	return r.store.ListSessions()
}

func (r *DefaultSessionRunner) ListTraces(limit int) []TraceEvent {
	return r.store.ListTraces(limit)
}

func (r *DefaultSessionRunner) ListModelJobs(limit int) []ModelJob {
	if tools, ok := r.tools.(interface{ ListModelJobs(int) []ModelJob }); ok {
		return tools.ListModelJobs(limit)
	}
	return nil
}

func (r *DefaultSessionRunner) ListIntakeWorkflows(ctx context.Context, limit int) ([]intakeapp.IntakeWorkflow, error) {
	if tools, ok := r.tools.(interface {
		ListIntakeWorkflows(context.Context, int) ([]intakeapp.IntakeWorkflow, error)
	}); ok {
		return tools.ListIntakeWorkflows(ctx, limit)
	}
	return nil, fmt.Errorf("intake workflow listing is not supported by this runtime")
}

func (r *DefaultSessionRunner) GetIntakeWorkflow(ctx context.Context, id string) (intakeapp.IntakeWorkflow, bool, error) {
	if tools, ok := r.tools.(interface {
		GetIntakeWorkflow(context.Context, string) (intakeapp.IntakeWorkflow, bool, error)
	}); ok {
		return tools.GetIntakeWorkflow(ctx, id)
	}
	return intakeapp.IntakeWorkflow{}, false, fmt.Errorf("intake workflow lookup is not supported by this runtime")
}

func (r *DefaultSessionRunner) ApproveIntakeWorkflow(ctx context.Context, id string, by string, note string) (intakeapp.IntakeWorkflow, error) {
	if tools, ok := r.tools.(interface {
		ApproveIntakeWorkflow(context.Context, string, string, string) (intakeapp.IntakeWorkflow, error)
	}); ok {
		return tools.ApproveIntakeWorkflow(ctx, id, by, note)
	}
	return intakeapp.IntakeWorkflow{}, fmt.Errorf("intake workflow approval is not supported by this runtime")
}

func (r *DefaultSessionRunner) RegisterIntakeWorkflow(ctx context.Context, id string, by string) (intakeapp.IntakeWorkflow, error) {
	if tools, ok := r.tools.(interface {
		RegisterIntakeWorkflow(context.Context, string, string) (intakeapp.IntakeWorkflow, error)
	}); ok {
		return tools.RegisterIntakeWorkflow(ctx, id, by)
	}
	return intakeapp.IntakeWorkflow{}, fmt.Errorf("intake workflow register is not supported by this runtime")
}

func (r *DefaultSessionRunner) GetModelJob(id string) (ModelJob, bool) {
	if tools, ok := r.tools.(interface{ GetModelJob(string) (ModelJob, bool) }); ok {
		return tools.GetModelJob(id)
	}
	return ModelJob{}, false
}

func (r *DefaultSessionRunner) LineageModelJob(id string) []ModelJob {
	if tools, ok := r.tools.(interface{ LineageModelJob(string) []ModelJob }); ok {
		return tools.LineageModelJob(id)
	}
	return nil
}

func (r *DefaultSessionRunner) CancelModelJob(id string) (ModelJob, error) {
	if tools, ok := r.tools.(interface {
		CancelModelJob(string) (ModelJob, error)
	}); ok {
		return tools.CancelModelJob(id)
	}
	return ModelJob{}, fmt.Errorf("model job cancellation is not supported by this runtime")
}

func (r *DefaultSessionRunner) ResumeModelJob(id string) (ModelJob, error) {
	if tools, ok := r.tools.(interface {
		ResumeModelJob(string) (ModelJob, error)
	}); ok {
		return tools.ResumeModelJob(id)
	}
	return ModelJob{}, fmt.Errorf("model job resume is not supported by this runtime")
}

func (r *DefaultSessionRunner) record(session SessionContext, msg channel.InboundMessage, intent Intent, calls []ToolCall, status string, reply string, errorText string, metadata map[string]string) {
	now := r.now()
	callToolIDs := collectToolIDs(calls)
	r.store.TouchSession(SessionState{
		Key:         session.Key,
		AgentID:     session.AgentID,
		Channel:     msg.Channel,
		AccountID:   msg.AccountID,
		PeerKind:    msg.Peer.Kind,
		PeerID:      msg.Peer.ID,
		SenderID:    msg.SenderID,
		LastIntent:  intent.Kind,
		LastToolIDs: callToolIDs,
		LastStatus:  status,
		UpdatedAt:   now,
	})
	r.store.RecordTrace(TraceEvent{
		ID:         fmt.Sprintf("trace-%d", now.UnixNano()),
		SessionKey: session.Key,
		MessageID:  msg.ID,
		Channel:    msg.Channel,
		AccountID:  msg.AccountID,
		PeerKind:   msg.Peer.Kind,
		PeerID:     msg.Peer.ID,
		SenderID:   msg.SenderID,
		Intent:     intent.Kind,
		AgentID:    session.AgentID,
		ToolIDs:    callToolIDs,
		Status:     status,
		ReplyText:  reply,
		Error:      errorText,
		Metadata:   metadata,
		CreatedAt:  now,
	})
}

func collectToolIDs(calls []ToolCall) []string {
	if len(calls) == 0 {
		return nil
	}
	out := make([]string, 0, len(calls))
	for _, call := range calls {
		if call.ToolID != "" {
			out = append(out, call.ToolID)
		}
	}
	return out
}

func hasToolCall(calls []ToolCall, toolID string) bool {
	for _, call := range calls {
		if call.ToolID == toolID {
			return true
		}
	}
	return false
}

func isExactToolPlan(calls []ToolCall, toolID string) bool {
	return len(calls) == 1 && calls[0].ToolID == toolID
}

func BuildSessionContext(msg channel.InboundMessage, delegation DelegationDecision) SessionContext {
	agentID := delegation.AgentID
	if agentID == "" {
		agentID = "go-control-plane"
	}
	return SessionContext{
		Key:       DefaultSessionKey(agentID, msg),
		AgentID:   agentID,
		Channel:   msg.Channel,
		AccountID: msg.AccountID,
		Peer:      msg.Peer,
		SenderID:  msg.SenderID,
		PeerKind:  msg.Peer.Kind,
		PeerID:    msg.Peer.ID,
	}
}

func DefaultSessionKey(agentID string, msg channel.InboundMessage) string {
	peerKind := msg.Peer.Kind
	if peerKind == "" {
		peerKind = channel.PeerKindDirect
	}
	peerID := msg.Peer.ID
	if peerID == "" {
		peerID = msg.SenderID
	}
	return fmt.Sprintf("agent:%s:%s:%s:%s", agentID, msg.Channel, peerKind, peerID)
}
