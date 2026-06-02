package labelctl

import "testing"

func TestNormalizeRuntimeInputDropsPowerShellBOM(t *testing.T) {
	got := normalizeRuntimeInput("\ufeff\ufeff/help\r\n")
	if got != "/help" {
		t.Fatalf("normalizeRuntimeInput() = %q, want /help", got)
	}
}

func TestCompactMetadataHandlesNonStringValues(t *testing.T) {
	got := compactMetadata(map[string]any{
		"complete": true,
		"count":    float64(3),
	})
	if got == "" {
		t.Fatal("compactMetadata() returned empty string")
	}
}

func TestToolProgressLinePrefersSingleToolID(t *testing.T) {
	got := toolProgressLine(runtimeStreamEvent{
		ToolID:  "runtime.health",
		ToolIDs: []string{"runtime.status"},
		Status:  "ok",
		Message: "tool_done: tool handler completed",
	})
	want := "  • tool=runtime.health status=ok tool_done: tool handler completed"
	if got != want {
		t.Fatalf("toolProgressLine() = %q, want %q", got, want)
	}
}
