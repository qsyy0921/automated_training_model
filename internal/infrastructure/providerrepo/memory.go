package providerrepo

import (
	"context"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/provider"
)

type MemoryRepository struct {
	mu        sync.Mutex
	providers map[string]provider.Provider
}

func NewMemoryRepository() *MemoryRepository {
	r := &MemoryRepository{providers: map[string]provider.Provider{}}
	r.providers["openai"] = provider.Provider{
		ID:           "openai",
		Type:         provider.ProviderOpenAI,
		DisplayName:  "OpenAI",
		BaseURL:      "https://api.openai.com/v1",
		APIKeyRef:    "env:OPENAI_API_KEY",
		DefaultModel: "gpt-4.1",
		VisionModel:  "gpt-4.1",
		Enabled:      true,
	}
	r.providers["qwen"] = provider.Provider{
		ID:           "qwen",
		Type:         provider.ProviderQwen,
		DisplayName:  "Qwen/DashScope",
		BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKeyRef:    "env:DASHSCOPE_API_KEY",
		DefaultModel: "qwen-plus",
		VisionModel:  "qwen-vl-max",
		Enabled:      true,
	}
	r.providers["openrouter"] = provider.Provider{
		ID:           "openrouter",
		Type:         provider.ProviderOpenRouter,
		DisplayName:  "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		APIKeyRef:    "env:OPENROUTER_API_KEY",
		DefaultModel: "openai/gpt-4.1",
		VisionModel:  "openai/gpt-4.1",
		Enabled:      true,
	}
	r.providers["mimo_anthropic"] = provider.Provider{
		ID:           "mimo_anthropic",
		Type:         provider.ProviderMimo,
		DisplayName:  "Mimo Anthropic-compatible",
		BaseURL:      "https://token-plan-cn.xiaomimimo.com/anthropic",
		APIKeyRef:    "env:ANTHROPIC_AUTH_TOKEN",
		DefaultModel: "mimo-v2.5-pro",
		VisionModel:  "mimo-v2.5",
		Enabled:      true,
	}
	r.providers["mimo_openai"] = provider.Provider{
		ID:           "mimo_openai",
		Type:         provider.ProviderMimo,
		DisplayName:  "Mimo OpenAI-compatible",
		BaseURL:      "https://token-plan-cn.xiaomimimo.com/v1",
		APIKeyRef:    "env:ANTHROPIC_AUTH_TOKEN",
		DefaultModel: "mimo-v2.5-pro",
		VisionModel:  "mimo-v2.5",
		Enabled:      true,
	}
	return r
}

func (r *MemoryRepository) ListProviders(ctx context.Context) ([]provider.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]provider.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out, nil
}

func (r *MemoryRepository) UpsertProvider(ctx context.Context, p provider.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID] = p
	return nil
}
