package labelctl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillDraftReviewCommandsUseSkillApp(t *testing.T) {
	root := t.TempDir()
	if err := runSkill([]string{"draft", "-id", "qq-data-intake", "-summary", "QQ attachment intake workflow", "-draft-root", root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "qq-data-intake", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := runSkill([]string{"drafts", "-draft-root", root}); err != nil {
		t.Fatal(err)
	}
	if err := runSkill([]string{"approve-draft", "qq-data-intake", "-draft-root", root, "-by", "tester", "-note", "reviewed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "qq-data-intake", "approval.json")); err != nil {
		t.Fatal(err)
	}
	if err := runSkill([]string{"reject-draft", "qq-data-intake", "-draft-root", root, "-by", "tester", "-note", "needs tests"}); err != nil {
		t.Fatal(err)
	}
}

func TestSkillDraftCommandRejectsSecretLikeSummary(t *testing.T) {
	err := runSkill([]string{"draft", "-id", "bad-secret", "-summary", "use tp-secret-token", "-draft-root", t.TempDir()})
	if err == nil {
		t.Fatal("expected secret-like skill summary to fail")
	}
}
