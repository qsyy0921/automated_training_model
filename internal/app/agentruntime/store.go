package agentruntime

import (
	"sort"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

const defaultTraceLimit = 100

type RuntimeStore interface {
	TouchSession(state SessionState)
	RecordTrace(event TraceEvent)
	Snapshot(limit int) RuntimeSnapshot
	ListSessions() []SessionState
	ListTraces(limit int) []TraceEvent
}

type SessionState struct {
	Key          string           `json:"key"`
	AgentID      string           `json:"agent_id"`
	Channel      channel.Kind     `json:"channel"`
	AccountID    string           `json:"account_id"`
	PeerKind     channel.PeerKind `json:"peer_kind"`
	PeerID       string           `json:"peer_id"`
	SenderID     string           `json:"sender_id"`
	MessageCount int              `json:"message_count"`
	LastIntent   IntentKind       `json:"last_intent,omitempty"`
	LastToolIDs  []string         `json:"last_tool_ids,omitempty"`
	LastStatus   string           `json:"last_status,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type TraceEvent struct {
	ID         string            `json:"id"`
	SessionKey string            `json:"session_key"`
	MessageID  string            `json:"message_id,omitempty"`
	Channel    channel.Kind      `json:"channel"`
	AccountID  string            `json:"account_id"`
	PeerKind   channel.PeerKind  `json:"peer_kind"`
	PeerID     string            `json:"peer_id"`
	SenderID   string            `json:"sender_id"`
	Intent     IntentKind        `json:"intent"`
	AgentID    string            `json:"agent_id"`
	ToolIDs    []string          `json:"tool_ids,omitempty"`
	Status     string            `json:"status"`
	ReplyText  string            `json:"reply_text,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

type RuntimeSnapshot struct {
	StartedAt    time.Time      `json:"started_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	SessionCount int            `json:"session_count"`
	TraceCount   int            `json:"trace_count"`
	Sessions     []SessionState `json:"sessions"`
	RecentTraces []TraceEvent   `json:"recent_traces"`
}

type InMemoryRuntimeStore struct {
	mu        sync.RWMutex
	startedAt time.Time
	updatedAt time.Time
	sessions  map[string]SessionState
	traces    []TraceEvent
}

func NewInMemoryRuntimeStore(now time.Time) *InMemoryRuntimeStore {
	if now.IsZero() {
		now = time.Now()
	}
	return &InMemoryRuntimeStore{
		startedAt: now,
		updatedAt: now,
		sessions:  map[string]SessionState{},
	}
}

func (s *InMemoryRuntimeStore) TouchSession(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.sessions[state.Key]
	if ok {
		state.CreatedAt = existing.CreatedAt
		state.MessageCount = existing.MessageCount + 1
	} else {
		state.CreatedAt = state.UpdatedAt
		state.MessageCount = 1
	}
	s.sessions[state.Key] = state
	s.updatedAt = state.UpdatedAt
}

func (s *InMemoryRuntimeStore) RecordTrace(event TraceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = append(s.traces, event)
	s.updatedAt = event.CreatedAt
}

func (s *InMemoryRuntimeStore) Snapshot(limit int) RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RuntimeSnapshot{
		StartedAt:    s.startedAt,
		UpdatedAt:    s.updatedAt,
		SessionCount: len(s.sessions),
		TraceCount:   len(s.traces),
		Sessions:     sortedSessions(s.sessions),
		RecentTraces: recentTraces(s.traces, normalizeTraceLimit(limit)),
	}
}

func (s *InMemoryRuntimeStore) ListSessions() []SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedSessions(s.sessions)
}

func (s *InMemoryRuntimeStore) ListTraces(limit int) []TraceEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return recentTraces(s.traces, normalizeTraceLimit(limit))
}

func sortedSessions(items map[string]SessionState) []SessionState {
	out := make([]SessionState, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func recentTraces(items []TraceEvent, limit int) []TraceEvent {
	if len(items) == 0 {
		return nil
	}
	start := len(items) - limit
	if start < 0 {
		start = 0
	}
	out := append([]TraceEvent(nil), items[start:]...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func normalizeTraceLimit(limit int) int {
	if limit <= 0 {
		return defaultTraceLimit
	}
	if limit > 500 {
		return 500
	}
	return limit
}
