package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/provider"
)

type EnvStore struct{}

func NewEnvStore() *EnvStore {
	return &EnvStore{}
}

func (s *EnvStore) PutAPIKey(ctx context.Context, providerID string, displayName string, plaintext string) (provider.APIKeySecret, error) {
	return provider.APIKeySecret{}, fmt.Errorf("env secret store is read-only; set provider keys through environment variables or a real secret backend")
}

func (s *EnvStore) GetAPIKey(ctx context.Context, ref string) (string, error) {
	name := strings.TrimPrefix(ref, "env:")
	if name == "" {
		return "", fmt.Errorf("empty env secret ref")
	}
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("environment variable is not set: %s", name)
	}
	return value, nil
}

func (s *EnvStore) ListAPIKeys(ctx context.Context) ([]provider.APIKeySecret, error) {
	candidates := []struct {
		ref        string
		providerID string
		name       string
	}{
		{"env:OPENAI_API_KEY", "openai", "OpenAI API Key"},
		{"env:ANTHROPIC_AUTH_TOKEN", "mimo_anthropic", "Mimo Anthropic-compatible Token"},
		{"env:DASHSCOPE_API_KEY", "qwen", "DashScope/Qwen API Key"},
		{"env:OPENROUTER_API_KEY", "openrouter", "OpenRouter API Key"},
	}
	out := []provider.APIKeySecret{}
	for _, c := range candidates {
		if value := os.Getenv(strings.TrimPrefix(c.ref, "env:")); value != "" {
			out = append(out, provider.APIKeySecret{
				Ref:         c.ref,
				ProviderID:  c.providerID,
				DisplayName: c.name,
				MaskedValue: mask(value),
			})
		}
	}
	return out, nil
}

func (s *EnvStore) DeleteAPIKey(ctx context.Context, ref string) error {
	return fmt.Errorf("env secret store is read-only")
}

func mask(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
