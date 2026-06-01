package agent

import "time"

type AgentStatus string

const (
	AgentStatusAvailable AgentStatus = "available"
	AgentStatusDisabled  AgentStatus = "disabled"
)

type AgentSpec struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	ToolIDs      []string          `json:"tool_ids,omitempty"`
	Runtime      string            `json:"runtime,omitempty"`
	PolicyTags   []string          `json:"policy_tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Status       AgentStatus       `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type ToolStatus string

const (
	ToolStatusAvailable ToolStatus = "available"
	ToolStatusDisabled  ToolStatus = "disabled"
)

type ToolSpec struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Version         string            `json:"version"`
	Description     string            `json:"description,omitempty"`
	InputSchemaURI  string            `json:"input_schema_uri,omitempty"`
	OutputSchemaURI string            `json:"output_schema_uri,omitempty"`
	PermissionLevel string            `json:"permission_level,omitempty"`
	Runtime         string            `json:"runtime,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Status          ToolStatus        `json:"status"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type WorkflowStatus string

const (
	WorkflowStatusAvailable WorkflowStatus = "available"
	WorkflowStatusDisabled  WorkflowStatus = "disabled"
)

type WorkflowSpec struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Trigger     string            `json:"trigger,omitempty"`
	AgentIDs    []string          `json:"agent_ids,omitempty"`
	ToolIDs     []string          `json:"tool_ids,omitempty"`
	Steps       []WorkflowStep    `json:"steps,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      WorkflowStatus    `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type WorkflowStep struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	AgentID    string            `json:"agent_id,omitempty"`
	ToolID     string            `json:"tool_id,omitempty"`
	Action     string            `json:"action"`
	Input      map[string]string `json:"input,omitempty"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	PolicyTags []string          `json:"policy_tags,omitempty"`
}

type RunRequest struct {
	WorkflowID string            `json:"workflow_id"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	Scene      string            `json:"scene,omitempty"`
	DryRun     bool              `json:"dry_run,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

type WorkflowRun struct {
	ID         string            `json:"id"`
	TaskID     string            `json:"task_id"`
	WorkflowID string            `json:"workflow_id"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	Scene      string            `json:"scene,omitempty"`
	Status     string            `json:"status"`
	Params     map[string]string `json:"params,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type AuditEvent struct {
	ID           string            `json:"id"`
	Actor        string            `json:"actor"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	RequestID    string            `json:"request_id,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}
