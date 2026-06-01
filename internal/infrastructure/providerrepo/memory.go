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
	r.providers["chat_default"] = provider.Provider{
		ID:           "chat_default",
		Type:         provider.ProviderChatCompatible,
		DisplayName:  "Generic Chat Endpoint",
		BaseURL:      "env:LLM_BASE_URL",
		APIKeyRef:    "env:LLM_API_KEY",
		DefaultModel: "env:LLM_MODEL",
		Enabled:      true,
	}
	r.providers["vision_default"] = provider.Provider{
		ID:           "vision_default",
		Type:         provider.ProviderVisionCompatible,
		DisplayName:  "Generic Vision Endpoint",
		BaseURL:      "env:VLM_BASE_URL",
		APIKeyRef:    "env:VLM_API_KEY",
		DefaultModel: "env:VLM_MODEL",
		VisionModel:  "env:VLM_MODEL",
		Enabled:      true,
	}
	r.providers["model_router"] = provider.Provider{
		ID:           "model_router",
		Type:         provider.ProviderModelRouter,
		DisplayName:  "Model Router",
		BaseURL:      "env:MODEL_ROUTER_BASE_URL",
		APIKeyRef:    "env:MODEL_ROUTER_API_KEY",
		DefaultModel: "env:MODEL_ROUTER_MODEL",
		VisionModel:  "env:MODEL_ROUTER_VISION_MODEL",
		Enabled:      true,
	}
	r.providers["local_runtime"] = provider.Provider{
		ID:           "local_runtime",
		Type:         provider.ProviderLocal,
		DisplayName:  "Local Model Runtime",
		BaseURL:      "env:LOCAL_MODEL_BASE_URL",
		APIKeyRef:    "env:LOCAL_MODEL_API_KEY",
		DefaultModel: "env:LOCAL_MODEL_ID",
		VisionModel:  "env:LOCAL_VISION_MODEL_ID",
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
