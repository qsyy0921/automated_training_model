package provider

type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderQwen       ProviderType = "qwen"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderMimo       ProviderType = "mimo"
	ProviderLocal      ProviderType = "local"
)

type Provider struct {
	ID           string       `json:"id"`
	Type         ProviderType `json:"type"`
	DisplayName  string       `json:"display_name"`
	BaseURL      string       `json:"base_url"`
	APIKeyRef    string       `json:"api_key_ref"`
	DefaultModel string       `json:"default_model"`
	VisionModel  string       `json:"vision_model,omitempty"`
	Enabled      bool         `json:"enabled"`
}

type APIKeySecret struct {
	Ref         string `json:"ref"`
	ProviderID  string `json:"provider_id"`
	DisplayName string `json:"display_name"`
	MaskedValue string `json:"masked_value"`
}
