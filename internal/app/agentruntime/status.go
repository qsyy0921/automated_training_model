package agentruntime

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
	Policy         string               `json:"policy"`
	EntryPoints    []EntryPointStatus   `json:"entry_points"`
	ProviderRoutes []ProviderRoute      `json:"provider_routes"`
	SubAgents      []SubAgentSpec       `json:"sub_agents"`
	SkillEvolution SkillEvolutionStatus `json:"skill_evolution"`
}

func Status() RuntimeStatus {
	return RuntimeStatus{
		Runtime:      "automated-training-agent-runtime",
		ControlPlane: "go-ddd-control-plane",
		AgentLoop:    "python-agent-runtime",
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
