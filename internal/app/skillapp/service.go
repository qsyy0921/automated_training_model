package skillapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Service struct {
	now func() time.Time
}

type DraftRequest struct {
	ID      string
	Title   string
	Summary string
	Root    string
}

type Draft struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Status       string    `json:"status"`
	Enabled      bool      `json:"enabled"`
	Path         string    `json:"path"`
	ApprovalPath string    `json:"approval_path,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type ReviewRequest struct {
	ID     string
	Root   string
	By     string
	Note   string
	Status string
}

type ReviewRecord struct {
	SkillID string    `json:"skill_id"`
	Status  string    `json:"status"`
	By      string    `json:"by"`
	Note    string    `json:"note,omitempty"`
	At      time.Time `json:"at"`
	Enabled bool      `json:"enabled"`
}

func NewService(now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{now: now}
}

func (s *Service) Draft(req DraftRequest) (Draft, error) {
	id := strings.TrimSpace(req.ID)
	if err := validateSkillID(id); err != nil {
		return Draft{}, err
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return Draft{}, fmt.Errorf("summary is required")
	}
	if LooksSecretLike(summary) {
		return Draft{}, fmt.Errorf("summary looks like it may contain a secret; remove tokens, keys, cookies, and raw private data before drafting a skill")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = id
	}
	root := defaultDraftRoot(req.Root)
	targetDir, err := safeChildDir(root, id)
	if err != nil {
		return Draft{}, err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return Draft{}, err
	}
	target := filepath.Join(targetDir, "SKILL.md")
	body := strings.Join([]string{
		"---",
		"name: " + id,
		"description: " + summary,
		"status: draft",
		"enabled: false",
		"---",
		"",
		"# " + title,
		"",
		"## Summary",
		"",
		summary,
		"",
		"## Safety",
		"",
		"- This skill is a draft and is not enabled automatically.",
		"- Review and remove secrets, private data, and environment-specific paths before promotion.",
		"- Promotion requires human approval and an audit event.",
		"",
		"## Workflow",
		"",
		"1. Describe the trigger condition.",
		"2. List required tools or MCP servers.",
		"3. Define approval gates and rollback behavior.",
		"4. Add focused tests before enabling.",
		"",
	}, "\n")
	if err := os.WriteFile(target, []byte(body), 0644); err != nil {
		return Draft{}, err
	}
	now := s.now()
	return Draft{ID: id, Title: title, Summary: summary, Status: "draft", Enabled: false, Path: target, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Service) List(root string) ([]Draft, error) {
	root = defaultDraftRoot(root)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	drafts := make([]Draft, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if err := validateSkillID(id); err != nil {
			continue
		}
		dir, err := safeChildDir(root, id)
		if err != nil {
			continue
		}
		skillPath := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}
		draft := Draft{ID: id, Title: id, Status: "draft", Enabled: false, Path: skillPath}
		if record, ok, err := readReviewRecord(dir); err != nil {
			return nil, err
		} else if ok {
			draft.Status = record.Status
			draft.Enabled = record.Enabled
			draft.ApprovalPath = filepath.Join(dir, "approval.json")
			draft.UpdatedAt = record.At
		}
		drafts = append(drafts, draft)
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].ID < drafts[j].ID })
	return drafts, nil
}

func (s *Service) Approve(req ReviewRequest) (ReviewRecord, error) {
	return s.review(req, "approved")
}

func (s *Service) Reject(req ReviewRequest) (ReviewRecord, error) {
	return s.review(req, "rejected")
}

func (s *Service) review(req ReviewRequest, status string) (ReviewRecord, error) {
	id := strings.TrimSpace(req.ID)
	if err := validateSkillID(id); err != nil {
		return ReviewRecord{}, err
	}
	root := defaultDraftRoot(req.Root)
	dir, err := safeChildDir(root, id)
	if err != nil {
		return ReviewRecord{}, err
	}
	skillPath := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		if os.IsNotExist(err) {
			return ReviewRecord{}, fmt.Errorf("skill draft not found: %s", id)
		}
		return ReviewRecord{}, err
	}
	by := strings.TrimSpace(req.By)
	if by == "" {
		by = "labelctl"
	}
	note := strings.TrimSpace(req.Note)
	if LooksSecretLike(note) {
		return ReviewRecord{}, fmt.Errorf("review note looks like it may contain a secret")
	}
	record := ReviewRecord{SkillID: id, Status: status, By: by, Note: note, At: s.now(), Enabled: false}
	target := filepath.Join(dir, "approval.json")
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ReviewRecord{}, err
	}
	if err := os.WriteFile(target, append(raw, '\n'), 0644); err != nil {
		return ReviewRecord{}, err
	}
	return record, nil
}

func LooksSecretLike(value string) bool {
	value = strings.ToLower(value)
	patterns := []string{"api_key", "apikey", "auth_token", "bearer ", "sk-", "tp-", "cookie=", "password="}
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func validateSkillID(id string) error {
	if id == "" {
		return fmt.Errorf("skill id is required")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(id) {
		return fmt.Errorf("invalid skill id: %s", id)
	}
	return nil
}

func defaultDraftRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return filepath.Join("data_lake", "agents", "skill_drafts")
	}
	return root
}

func safeChildDir(root string, id string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absRoot, id)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("skill draft path escapes draft root")
	}
	return absTarget, nil
}

func readReviewRecord(dir string) (ReviewRecord, bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if os.IsNotExist(err) {
		return ReviewRecord{}, false, nil
	}
	if err != nil {
		return ReviewRecord{}, false, err
	}
	var record ReviewRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return ReviewRecord{}, false, err
	}
	return record, true, nil
}
