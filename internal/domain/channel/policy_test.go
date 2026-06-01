package channel

import "testing"

func TestSessionKey(t *testing.T) {
	key := SessionKey("main", Peer{Channel: KindQQ, Kind: PeerKindGroup, ID: "group-openid"})
	if key != "agent:main:qq:group:group-openid" {
		t.Fatalf("unexpected session key: %s", key)
	}
}

func TestRequiresApproval(t *testing.T) {
	if RequiresApproval(DataIntakePlan{DryRun: true, RiskLevel: "low"}) {
		t.Fatal("low-risk dry-run should not require approval")
	}
	if !RequiresApproval(DataIntakePlan{DryRun: false, RiskLevel: "low"}) {
		t.Fatal("non-dry-run should require approval")
	}
	if !RequiresApproval(DataIntakePlan{DryRun: true, RiskLevel: "high"}) {
		t.Fatal("high-risk plan should require approval")
	}
}
