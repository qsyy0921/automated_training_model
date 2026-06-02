package runtimerepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
)

type JSONRuntimeStore struct {
	mu           sync.RWMutex
	root         string
	startedAt    time.Time
	updatedAt    time.Time
	sessions     map[string]agentruntime.SessionState
	traces       []agentruntime.TraceEvent
	maxTraceKeep int
}

type metaFile struct {
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewJSONRuntimeStore(root string, now time.Time) (*JSONRuntimeStore, error) {
	if now.IsZero() {
		now = time.Now()
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	store := &JSONRuntimeStore{
		root:         root,
		startedAt:    now,
		updatedAt:    now,
		sessions:     map[string]agentruntime.SessionState{},
		maxTraceKeep: 5000,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *JSONRuntimeStore) TouchSession(state agentruntime.SessionState) {
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
	_ = s.persistLocked()
}

func (s *JSONRuntimeStore) RecordTrace(event agentruntime.TraceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = append(s.traces, event)
	if len(s.traces) > s.maxTraceKeep {
		s.traces = append([]agentruntime.TraceEvent(nil), s.traces[len(s.traces)-s.maxTraceKeep:]...)
	}
	s.updatedAt = event.CreatedAt
	_ = s.persistLocked()
}

func (s *JSONRuntimeStore) Snapshot(limit int) agentruntime.RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return agentruntime.RuntimeSnapshot{
		StartedAt:    s.startedAt,
		UpdatedAt:    s.updatedAt,
		SessionCount: len(s.sessions),
		TraceCount:   len(s.traces),
		Sessions:     sortedSessions(s.sessions),
		RecentTraces: recentTraces(s.traces, normalizeTraceLimit(limit)),
	}
}

func (s *JSONRuntimeStore) ListSessions() []agentruntime.SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedSessions(s.sessions)
}

func (s *JSONRuntimeStore) ListTraces(limit int) []agentruntime.TraceEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return recentTraces(s.traces, normalizeTraceLimit(limit))
}

func (s *JSONRuntimeStore) load() error {
	var meta metaFile
	if err := readJSONFile(s.metaPath(), &meta); err != nil {
		return err
	}
	if !meta.StartedAt.IsZero() {
		s.startedAt = meta.StartedAt
	}
	if !meta.UpdatedAt.IsZero() {
		s.updatedAt = meta.UpdatedAt
	}
	var sessions []agentruntime.SessionState
	if err := readJSONFile(s.sessionsPath(), &sessions); err != nil {
		return err
	}
	for _, item := range sessions {
		if item.Key != "" {
			s.sessions[item.Key] = item
		}
	}
	if err := readJSONFile(s.tracesPath(), &s.traces); err != nil {
		return err
	}
	if len(s.traces) > s.maxTraceKeep {
		s.traces = append([]agentruntime.TraceEvent(nil), s.traces[len(s.traces)-s.maxTraceKeep:]...)
	}
	return nil
}

func (s *JSONRuntimeStore) persistLocked() error {
	if err := writeJSONFile(s.metaPath(), metaFile{StartedAt: s.startedAt, UpdatedAt: s.updatedAt}); err != nil {
		return err
	}
	if err := writeJSONFile(s.sessionsPath(), sortedSessions(s.sessions)); err != nil {
		return err
	}
	return writeJSONFile(s.tracesPath(), s.traces)
}

func (s *JSONRuntimeStore) metaPath() string {
	return filepath.Join(s.root, "runtime_meta.json")
}

func (s *JSONRuntimeStore) sessionsPath() string {
	return filepath.Join(s.root, "runtime_sessions.json")
}

func (s *JSONRuntimeStore) tracesPath() string {
	return filepath.Join(s.root, "runtime_traces.json")
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, value)
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmpPath, path)
}

func sortedSessions(items map[string]agentruntime.SessionState) []agentruntime.SessionState {
	out := make([]agentruntime.SessionState, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func recentTraces(items []agentruntime.TraceEvent, limit int) []agentruntime.TraceEvent {
	if len(items) == 0 {
		return nil
	}
	start := len(items) - limit
	if start < 0 {
		start = 0
	}
	out := append([]agentruntime.TraceEvent(nil), items[start:]...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func normalizeTraceLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
