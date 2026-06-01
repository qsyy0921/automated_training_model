package channelapp

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type Repository interface {
	ListAccounts(ctx context.Context, kind channel.Kind) ([]channel.Account, error)
	GetAccount(ctx context.Context, kind channel.Kind, id string) (*channel.Account, error)
	SaveAccount(ctx context.Context, account channel.Account) (channel.Account, error)
	AppendEvent(ctx context.Context, event channel.Event) error
	ListEvents(ctx context.Context, kind channel.Kind, limit int) ([]channel.Event, error)
}

type AgentIngress interface {
	HandleChannelMessage(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error)
}

type Runtime interface {
	Start(ctx context.Context, account channel.Account) error
	Stop(ctx context.Context, accountID string) error
	Status(ctx context.Context, accountID string) (channel.AccountStatus, error)
}
