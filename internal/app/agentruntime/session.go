package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	return &DefaultSessionRunner{planner: planner, tools: tools, store: store, now: now}
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
		r.record(session, msg, intent, nil, "ok", reply.Text, "")
		return reply, nil
	}

	plan, err := r.planner.Plan(ctx, PlanRequest{
		Message:    msg,
		Session:    session,
		Intent:     intent,
		Delegation: delegation,
	})
	if err != nil {
		r.record(session, msg, intent, nil, "planning_failed", "", err.Error())
		return channel.OutboundMessage{}, err
	}
	if len(plan.ToolCalls) == 0 {
		reply.Text = plan.ReplyText
		r.record(session, msg, plan.Intent, nil, plan.Status, reply.Text, "")
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
		r.record(session, msg, plan.Intent, plan.ToolCalls, "tool_failed", "", err.Error())
		return channel.OutboundMessage{}, err
	}
	reply.Text = result.ReplyText
	r.record(session, msg, plan.Intent, plan.ToolCalls, result.Status, reply.Text, "")
	return reply, nil
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

func (r *DefaultSessionRunner) record(session SessionContext, msg channel.InboundMessage, intent Intent, calls []ToolCall, status string, reply string, errorText string) {
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
