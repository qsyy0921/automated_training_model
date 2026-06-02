package agentruntime

import (
	"errors"
	"testing"
)

func TestErrorEnvelopeFromStatusMapsRuntimeCodes(t *testing.T) {
	envelope := ErrorEnvelopeFromStatus("planning_failed", "planner-agent", errors.New("planner unavailable"))
	if envelope.Code != "runtime.planning_failed" || envelope.Source != "planner-agent" || !envelope.Retryable {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestErrorEnvelopeFromHTTPStatusMapsGatewayCodes(t *testing.T) {
	envelope := ErrorEnvelopeFromHTTPStatus(404, "model job not found")
	if envelope.Code != "gateway.not_found" || envelope.Source != "gateway" || envelope.Retryable {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}
