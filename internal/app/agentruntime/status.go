package agentruntime

import (
	"os"
	"path/filepath"
	"strings"
)

type ProviderRoute struct {
	ID        string `json:"id"`
	UseCase   string `json:"use_case"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	SecretRef string `json:"secret_ref,omitempty"`
	Boundary  string `json:"boundary"`
}

type EntryPointStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Transport   string `json:"transport"`
	Status      string `json:"status"`
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description,omitempty"`
}

type SkillEvolutionStatus struct {
	EnabledByDefault bool     `json:"enabled_by_default"`
	CurrentMode      string   `json:"current_mode"`
	Controls         []string `json:"controls"`
}

type RuntimeStatus struct {
	Runtime        string               `json:"runtime"`
	ControlPlane   string               `json:"control_plane"`
	AgentLoop      string               `json:"agent_loop"`
	Planner        PlannerStatus        `json:"planner"`
	Policy         string               `json:"policy"`
	EntryPoints    []EntryPointStatus   `json:"entry_points"`
	ProviderRoutes []ProviderRoute      `json:"provider_routes"`
	SubAgents      []SubAgentSpec       `json:"sub_agents"`
	SkillEvolution SkillEvolutionStatus `json:"skill_evolution"`
}

type PlannerStatus struct {
	Mode          string `json:"mode"`
	MimoEnabled   bool   `json:"mimo_enabled"`
	MimoFallback  string `json:"mimo_fallback"`
	Python        string `json:"python,omitempty"`
	PythonPath    string `json:"python_path,omitempty"`
	TextModel     string `json:"text_model"`
	VisionModel   string `json:"vision_model"`
	TokenPresent  bool   `json:"token_present"`
	EffectiveMode string `json:"effective_mode"`
}

func Status() RuntimeStatus {
	return RuntimeStatus{
		Runtime:      "automated-training-agent-runtime",
		ControlPlane: "go-ddd-control-plane",
		AgentLoop:    "python-agent-runtime",
		Planner:      plannerStatusFromEnv(),
		Policy:       "gateway-routed, approval-before-side-effects, channel-origin-audited",
		EntryPoints: []EntryPointStatus{
			{ID: "cli", Name: "CLI", Transport: "local-command", Status: "ready", Description: "labelctl runtime, agent, workflow, governance and channel commands"},
			{ID: "web", Name: "Web Console", Transport: "http", Status: "ready", Endpoint: "/", Description: "Agent overview, review workbench, tasks and governance"},
			{ID: "desktop", Name: "Desktop Client", Transport: "local-http", Status: "scaffolded", Endpoint: "/api/runtime/status", Description: "local desktop shell entry reuses the same Gateway APIs"},
			{ID: "qq", Name: "QQ / NapCat OneBot", Transport: "webhook", Status: "adapter-ready", Endpoint: "/api/channels/qq/onebot", Description: "QQ messages are normalized into channel.InboundMessage"},
		},
		ProviderRoutes: []ProviderRoute{
			{ID: "text-planning", UseCase: "intent, planning, workflow reasoning, JSON plan", Provider: "mimo", Model: "mimo-v2.5-pro", SecretRef: "env:LLM_API_KEY or env:ANTHROPIC_AUTH_TOKEN", Boundary: "server-side only"},
			{ID: "vision", UseCase: "image inspection and visual data understanding", Provider: "mimo", Model: "mimo-v2.5", SecretRef: "env:VLM_API_KEY or env:ANTHROPIC_AUTH_TOKEN", Boundary: "server-side only"},
			{ID: "image-generation", UseCase: "architecture images and generated visual assets", Provider: "chatgpt-reverse-proxy", Model: "chatgpt-5.5-image", SecretRef: "mcp:image-generation-proxy", Boundary: "MCP tool adapter; skill only stores usage recipe"},
		},
		SubAgents: DefaultSubAgents(),
		SkillEvolution: SkillEvolutionStatus{
			EnabledByDefault: false,
			CurrentMode:      "disabled",
			Controls: []string{
				"observe successful traces only",
				"draft SKILL.md into quarantine",
				"require human approval before enablement",
				"never copy secrets or raw private data into skills",
			},
		},
	}
}

func plannerStatusFromEnv() PlannerStatus {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PLANNER")))
	if mode == "" {
		mode = "auto"
	}
	mimoEnabled := truthy(os.Getenv("AGENT_RUNTIME_USE_MIMO"))
	effective := "rule"
	if mode == "python" || (mode == "auto" && mimoEnabled) {
		effective = "python"
	}
	python := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHON"))
	if python == "" {
		python = "python"
	}
	pythonPath := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHONPATH"))
	if pythonPath == "" {
		pythonPath = filepath.Join("workers", "python")
	}
	return PlannerStatus{
		Mode:          mode,
		MimoEnabled:   mimoEnabled,
		MimoFallback:  valueOrEnv("AGENT_RUNTIME_MIMO_FALLBACK", "rule"),
		Python:        python,
		PythonPath:    pythonPath,
		TextModel:     firstRuntimeEnv("MIMO_DEFAULT_MODEL", "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "mimo-v2.5-pro"),
		VisionModel:   firstRuntimeEnv("MIMO_VISION_MODEL", "VLM_MODEL", "ANTHROPIC_VISION_MODEL", "mimo-v2.5"),
		TokenPresent:  strings.TrimSpace(os.Getenv("ANTHROPIC_AUTH_TOKEN")) != "",
		EffectiveMode: effective,
	}
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func valueOrEnv(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func firstRuntimeEnv(names ...string) string {
	if len(names) == 0 {
		return ""
	}
	fallback := names[len(names)-1]
	for _, name := range names[:len(names)-1] {
		value := strings.TrimSpace(os.Getenv(name))
		if value != "" {
			return value
		}
	}
	return fallback
}
