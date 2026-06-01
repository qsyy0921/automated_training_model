package channelapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type Service struct {
	repo    Repository
	ingress AgentIngress
	runtime Runtime
}

func NewService(repo Repository, ingress AgentIngress, runtime Runtime) *Service {
	return &Service{repo: repo, ingress: ingress, runtime: runtime}
}

func (s *Service) ListAccounts(ctx context.Context, kind channel.Kind) ([]channel.Account, error) {
	return s.repo.ListAccounts(ctx, kind)
}

func (s *Service) SaveAccount(ctx context.Context, account channel.Account) (channel.Account, error) {
	now := time.Now()
	account.ID = strings.TrimSpace(account.ID)
	if account.ID == "" {
		account.ID = "default"
	}
	if account.Channel == "" {
		return channel.Account{}, fmt.Errorf("channel is required")
	}
	if account.DefaultAgentID == "" {
		account.DefaultAgentID = "main"
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	account.UpdatedAt = now
	return s.repo.SaveAccount(ctx, account)
}

func (s *Service) HandleInbound(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error) {
	if msg.Channel == "" {
		return channel.OutboundMessage{}, fmt.Errorf("channel is required")
	}
	if strings.TrimSpace(msg.AccountID) == "" {
		return channel.OutboundMessage{}, fmt.Errorf("account_id is required")
	}
	if strings.TrimSpace(msg.ID) == "" {
		return channel.OutboundMessage{}, fmt.Errorf("message id is required")
	}
	return s.ingress.HandleChannelMessage(ctx, msg)
}

func (s *Service) StartAccount(ctx context.Context, kind channel.Kind, id string) error {
	account, err := s.repo.GetAccount(ctx, kind, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	return s.runtime.Start(ctx, *account)
}

func (s *Service) StopAccount(ctx context.Context, accountID string) error {
	return s.runtime.Stop(ctx, strings.TrimSpace(accountID))
}

func (s *Service) AccountStatus(ctx context.Context, accountID string) (channel.AccountStatus, error) {
	return s.runtime.Status(ctx, strings.TrimSpace(accountID))
}
