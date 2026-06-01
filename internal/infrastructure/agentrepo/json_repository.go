package agentrepo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
)

type JSONRepository struct {
	root          string
	agentsPath    string
	toolsPath     string
	workflowsPath string
	runsPath      string
	auditPath     string
	mu            sync.Mutex
}

func NewJSONRepository(root string) *JSONRepository {
	return &JSONRepository{
		root:          root,
		agentsPath:    filepath.Join(root, "agents.json"),
		toolsPath:     filepath.Join(root, "tools.json"),
		workflowsPath: filepath.Join(root, "workflows.json"),
		runsPath:      filepath.Join(root, "runs.json"),
		auditPath:     filepath.Join(root, "audit.jsonl"),
	}
}

func (r *JSONRepository) BootstrapDefaults(ctx context.Context) error {
	now := time.Now()
	agents, err := r.ListAgents(ctx)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		for _, row := range agent.DefaultAgents(now) {
			if _, err := r.SaveAgent(ctx, row); err != nil {
				return err
			}
		}
	}
	tools, err := r.ListTools(ctx)
	if err != nil {
		return err
	}
	if len(tools) == 0 {
		for _, row := range agent.DefaultTools(now) {
			if _, err := r.SaveTool(ctx, row); err != nil {
				return err
			}
		}
	}
	workflows, err := r.ListWorkflows(ctx)
	if err != nil {
		return err
	}
	if len(workflows) == 0 {
		for _, row := range agent.DefaultWorkflows(now) {
			if _, err := r.SaveWorkflow(ctx, row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *JSONRepository) ListAgents(ctx context.Context) ([]agent.AgentSpec, error) {
	rows, err := readJSON[agent.AgentSpec](r.agentsPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func (r *JSONRepository) GetAgent(ctx context.Context, id string) (*agent.AgentSpec, error) {
	rows, err := readJSON[agent.AgentSpec](r.agentsPath)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			copied := row
			return &copied, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

func (r *JSONRepository) SaveAgent(ctx context.Context, spec agent.AgentSpec) (agent.AgentSpec, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := readJSON[agent.AgentSpec](r.agentsPath)
	if err != nil {
		return agent.AgentSpec{}, err
	}
	rows = upsert(rows, spec, func(row agent.AgentSpec) string { return row.ID })
	return spec, writeJSON(r.agentsPath, rows)
}

func (r *JSONRepository) ListTools(ctx context.Context) ([]agent.ToolSpec, error) {
	rows, err := readJSON[agent.ToolSpec](r.toolsPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func (r *JSONRepository) GetTool(ctx context.Context, id string) (*agent.ToolSpec, error) {
	rows, err := readJSON[agent.ToolSpec](r.toolsPath)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			copied := row
			return &copied, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", id)
}

func (r *JSONRepository) SaveTool(ctx context.Context, spec agent.ToolSpec) (agent.ToolSpec, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := readJSON[agent.ToolSpec](r.toolsPath)
	if err != nil {
		return agent.ToolSpec{}, err
	}
	rows = upsert(rows, spec, func(row agent.ToolSpec) string { return row.ID })
	return spec, writeJSON(r.toolsPath, rows)
}

func (r *JSONRepository) ListWorkflows(ctx context.Context) ([]agent.WorkflowSpec, error) {
	rows, err := readJSON[agent.WorkflowSpec](r.workflowsPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func (r *JSONRepository) GetWorkflow(ctx context.Context, id string) (*agent.WorkflowSpec, error) {
	rows, err := readJSON[agent.WorkflowSpec](r.workflowsPath)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			copied := row
			return &copied, nil
		}
	}
	return nil, fmt.Errorf("workflow not found: %s", id)
}

func (r *JSONRepository) SaveWorkflow(ctx context.Context, spec agent.WorkflowSpec) (agent.WorkflowSpec, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := readJSON[agent.WorkflowSpec](r.workflowsPath)
	if err != nil {
		return agent.WorkflowSpec{}, err
	}
	rows = upsert(rows, spec, func(row agent.WorkflowSpec) string { return row.ID })
	return spec, writeJSON(r.workflowsPath, rows)
}

func (r *JSONRepository) ListRuns(ctx context.Context) ([]agent.WorkflowRun, error) {
	rows, err := readJSON[agent.WorkflowRun](r.runsPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows, nil
}

func (r *JSONRepository) SaveRun(ctx context.Context, run agent.WorkflowRun) (agent.WorkflowRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := readJSON[agent.WorkflowRun](r.runsPath)
	if err != nil {
		return agent.WorkflowRun{}, err
	}
	rows = upsert(rows, run, func(row agent.WorkflowRun) string { return row.ID })
	return run, writeJSON(r.runsPath, rows)
}

func (r *JSONRepository) ListAuditEvents(ctx context.Context, limit int) ([]agent.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	file, err := os.Open(r.auditPath)
	if os.IsNotExist(err) {
		return []agent.AuditEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows := []agent.AuditEvent{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row agent.AuditEvent
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse audit event: %w", err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (r *JSONRepository) AppendAuditEvent(ctx context.Context, event agent.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.auditPath), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(r.auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func upsert[T any](rows []T, row T, id func(T) string) []T {
	for i := range rows {
		if id(rows[i]) == id(row) {
			rows[i] = row
			return rows
		}
	}
	return append(rows, row)
}

func readJSON[T any](path string) ([]T, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []T{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []T{}, nil
	}
	var rows []T
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return rows, nil
}

func writeJSON[T any](path string, rows []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + "." + strconv.FormatInt(time.Now().UnixNano(), 10) + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0644); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmp, path)
}
