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
