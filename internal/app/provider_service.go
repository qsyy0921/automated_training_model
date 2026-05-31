package app

import (
	"context"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/provider"
)

type ProviderRepository interface {
	ListProviders(ctx context.Context) ([]provider.Provider, error)
	UpsertProvider(ctx context.Context, p provider.Provider) error
}

type SecretStore interface {
	PutAPIKey(ctx context.Context, providerID string, displayName string, plaintext string) (provider.APIKeySecret, error)
	GetAPIKey(ctx context.Context, ref string) (string, error)
	ListAPIKeys(ctx context.Context) ([]provider.APIKeySecret, error)
	DeleteAPIKey(ctx context.Context, ref string) error
}

type ProviderService struct {
	providers ProviderRepository
	secrets   SecretStore
}

func NewProviderService(providers ProviderRepository, secrets SecretStore) *ProviderService {
	return &ProviderService{providers: providers, secrets: secrets}
}

func (s *ProviderService) ListProviders(ctx context.Context) ([]provider.Provider, error) {
	return s.providers.ListProviders(ctx)
}

func (s *ProviderService) SaveProvider(ctx context.Context, p provider.Provider) error {
	return s.providers.UpsertProvider(ctx, p)
}

func (s *ProviderService) SaveAPIKey(ctx context.Context, providerID string, displayName string, plaintext string) (provider.APIKeySecret, error) {
	return s.secrets.PutAPIKey(ctx, providerID, displayName, plaintext)
}

func (s *ProviderService) ListAPIKeys(ctx context.Context) ([]provider.APIKeySecret, error) {
	return s.secrets.ListAPIKeys(ctx)
}
