package channel

import "time"

type Kind string

const (
	KindQQ Kind = "qq"
)

type PeerKind string

const (
	PeerKindDirect  PeerKind = "direct"
	PeerKindGroup   PeerKind = "group"
	PeerKindChannel PeerKind = "channel"
)

type ToolPolicy string

const (
	ToolPolicyNone       ToolPolicy = "none"
	ToolPolicyRestricted ToolPolicy = "restricted"
	ToolPolicyFull       ToolPolicy = "full"
)

type Account struct {
	ID             string            `json:"id"`
	Channel        Kind              `json:"channel"`
	Name           string            `json:"name,omitempty"`
	Enabled        bool              `json:"enabled"`
	CredentialRef  string            `json:"credential_ref,omitempty"`
	DefaultAgentID string            `json:"default_agent_id,omitempty"`
	AllowFrom      []string          `json:"allow_from,omitempty"`
	GroupAllowFrom []string          `json:"group_allow_from,omitempty"`
	GroupPolicy    string            `json:"group_policy,omitempty"`
	Groups         map[string]Group  `json:"groups,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Group struct {
	Name           string     `json:"name,omitempty"`
	RequireMention bool       `json:"require_mention"`
	HistoryLimit   int        `json:"history_limit,omitempty"`
	ToolPolicy     ToolPolicy `json:"tool_policy,omitempty"`
	Prompt         string     `json:"prompt,omitempty"`
}

type Peer struct {
	Channel   Kind     `json:"channel"`
	AccountID string   `json:"account_id"`
	Kind      PeerKind `json:"kind"`
	ID        string   `json:"id"`
	Name      string   `json:"name,omitempty"`
}

type AttachmentStatus string

const (
	AttachmentReceived    AttachmentStatus = "received"
	AttachmentQuarantined AttachmentStatus = "quarantined"
	AttachmentScanned     AttachmentStatus = "scanned"
	AttachmentAccepted    AttachmentStatus = "accepted"
	AttachmentRejected    AttachmentStatus = "rejected"
)

type Attachment struct {
	ID        string           `json:"id"`
	Name      string           `json:"name,omitempty"`
	MediaType string           `json:"media_type,omitempty"`
	SizeBytes int64            `json:"size_bytes,omitempty"`
	SourceURI string           `json:"source_uri,omitempty"`
	LocalURI  string           `json:"local_uri,omitempty"`
	SHA256    string           `json:"sha256,omitempty"`
	Status    AttachmentStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
}

type InboundMessage struct {
	ID          string       `json:"id"`
	Channel     Kind         `json:"channel"`
	AccountID   string       `json:"account_id"`
	Peer        Peer         `json:"peer"`
	SenderID    string       `json:"sender_id"`
	SenderName  string       `json:"sender_name,omitempty"`
	Text        string       `json:"text,omitempty"`
	Mentioned   bool         `json:"mentioned,omitempty"`
	ReplyToID   string       `json:"reply_to_id,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ReceivedAt  time.Time    `json:"received_at"`
}

type OutboundMessage struct {
	Channel   Kind   `json:"channel"`
	AccountID string `json:"account_id"`
	Peer      Peer   `json:"peer"`
	Text      string `json:"text,omitempty"`
	ReplyToID string `json:"reply_to_id,omitempty"`
}

type AccountStatus struct {
	AccountID      string    `json:"account_id"`
	Channel        Kind      `json:"channel"`
	Enabled        bool      `json:"enabled"`
	Configured     bool      `json:"configured"`
	Connected      bool      `json:"connected"`
	LastInboundAt  time.Time `json:"last_inbound_at,omitempty"`
	LastOutboundAt time.Time `json:"last_outbound_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Event struct {
	ID         string            `json:"id"`
	Channel    Kind              `json:"channel"`
	AccountID  string            `json:"account_id"`
	PeerKind   PeerKind          `json:"peer_kind,omitempty"`
	PeerID     string            `json:"peer_id,omitempty"`
	SenderID   string            `json:"sender_id,omitempty"`
	Action     string            `json:"action"`
	Decision   string            `json:"decision,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	OccurredAt time.Time         `json:"occurred_at"`
}

type IntakeIntent string

const (
	IntakeIntentInspect         IntakeIntent = "inspect"
	IntakeIntentRegisterDataset IntakeIntent = "register_dataset"
	IntakeIntentUploadArchive   IntakeIntent = "upload_archive"
	IntakeIntentCreateReview    IntakeIntent = "create_review_task"
	IntakeIntentAskFollowUp     IntakeIntent = "ask_followup"
)

type PlannedAction struct {
	Kind   string            `json:"kind"`
	Params map[string]string `json:"params,omitempty"`
}

type DataIntakePlan struct {
	ID                string          `json:"id"`
	SourceMessageID   string          `json:"source_message_id"`
	Channel           Kind            `json:"channel"`
	AccountID         string          `json:"account_id"`
	SenderID          string          `json:"sender_id"`
	Intent            IntakeIntent    `json:"intent"`
	DatasetName       string          `json:"dataset_name,omitempty"`
	ProposedActions   []PlannedAction `json:"proposed_actions,omitempty"`
	RequiredApprovals []string        `json:"required_approvals,omitempty"`
	RiskLevel         string          `json:"risk_level,omitempty"`
	DryRun            bool            `json:"dry_run"`
	CreatedAt         time.Time       `json:"created_at"`
}
