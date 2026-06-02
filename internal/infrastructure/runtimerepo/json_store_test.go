package runtimerepo

import (
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestJSONRuntimeStoreRestoresSessionsAndTraces(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store, err := NewJSONRuntimeStore(root, now)
	if err != nil {
		t.Fatal(err)
	}
	session := agentruntime.SessionState{
		Key:        "agent:planner-agent:qq:direct:10001",
		AgentID:    "planner-agent",
		Channel:    channel.KindQQ,
		AccountID:  "default",
		PeerKind:   channel.PeerKindDirect,
		PeerID:     "10001",
		SenderID:   "10001",
		LastIntent: agentruntime.IntentChat,
		LastStatus: "planned",
		UpdatedAt:  now,
	}
	store.TouchSession(session)
	store.RecordTrace(agentruntime.TraceEvent{
		ID:         "trace-1",
		SessionKey: session.Key,
		Channel:    channel.KindQQ,
		AccountID:  "default",
		PeerKind:   channel.PeerKindDirect,
		PeerID:     "10001",
		SenderID:   "10001",
		Intent:     agentruntime.IntentChat,
		AgentID:    "planner-agent",
		ToolIDs:    []string{"llm.plan"},
		Status:     "planned",
		ReplyText:  "已规划。",
		CreatedAt:  now,
	})

	restored, err := NewJSONRuntimeStore(root, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := restored.Snapshot(10)
	if snapshot.SessionCount != 1 || snapshot.TraceCount != 1 {
		t.Fatalf("expected restored session and trace, got %+v", snapshot)
	}
	if snapshot.Sessions[0].MessageCount != 1 {
		t.Fatalf("unexpected message count after restore: %+v", snapshot.Sessions[0])
	}
	if snapshot.RecentTraces[0].ToolIDs[0] != "llm.plan" {
		t.Fatalf("unexpected restored trace: %+v", snapshot.RecentTraces[0])
	}

	session.UpdatedAt = now.Add(time.Minute)
	restored.TouchSession(session)
	again, err := NewJSONRuntimeStore(root, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	sessions := again.ListSessions()
	if len(sessions) != 1 || sessions[0].MessageCount != 2 {
		t.Fatalf("expected message count to persist as 2, got %+v", sessions)
	}
}
