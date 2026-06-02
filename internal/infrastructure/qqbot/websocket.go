package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketConfig struct {
	Enabled        bool          `json:"enabled"`
	URL            string        `json:"url,omitempty"`
	AccountID      string        `json:"account_id,omitempty"`
	AccessToken    string        `json:"-"`
	ReconnectDelay time.Duration `json:"-"`
}

type WebSocketStatus struct {
	Enabled   bool   `json:"enabled"`
	URL       string `json:"url,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

type WebSocketEventHandler func(context.Context, OneBotEvent) (OneBotReply, bool, error)

func WebSocketConfigFromEnv() WebSocketConfig {
	return WebSocketConfig{
		Enabled:        parseBool(os.Getenv("QQ_ONEBOT_WS_ENABLED")),
		URL:            strings.TrimSpace(os.Getenv("QQ_ONEBOT_WS_URL")),
		AccountID:      valueOr(strings.TrimSpace(os.Getenv("QQ_ONEBOT_ACCOUNT_ID")), "default"),
		AccessToken:    strings.TrimSpace(os.Getenv("QQ_ONEBOT_ACCESS_TOKEN")),
		ReconnectDelay: 3 * time.Second,
	}
}

func WebSocketStatusFromEnv() WebSocketStatus {
	cfg := WebSocketConfigFromEnv()
	return WebSocketStatus{Enabled: cfg.Enabled, URL: cfg.URL, AccountID: cfg.AccountID}
}

func RunWebSocketClient(ctx context.Context, cfg WebSocketConfig, handle WebSocketEventHandler) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("QQ_ONEBOT_WS_URL is required when QQ_ONEBOT_WS_ENABLED=true")
	}
	if handle == nil {
		return fmt.Errorf("onebot websocket handler is required")
	}
	delay := cfg.ReconnectDelay
	if delay <= 0 {
		delay = 3 * time.Second
	}
	for {
		if err := runWebSocketOnce(ctx, cfg, handle); err != nil {
			if ctx.Err() != nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
	}
}

func runWebSocketOnce(ctx context.Context, cfg WebSocketConfig, handle WebSocketEventHandler) error {
	headers := http.Header{}
	if cfg.AccessToken != "" {
		headers.Set("Authorization", "Bearer "+cfg.AccessToken)
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, cfg.URL, headers)
	if err != nil {
		return err
	}
	defer conn.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var event OneBotEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			continue
		}
		if event.PostType != "message" {
			continue
		}
		reply, shouldReply, err := handle(ctx, event)
		if err != nil || !shouldReply || reply.Action == "" {
			continue
		}
		if err := conn.WriteJSON(reply); err != nil {
			return err
		}
	}
}

func valueOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
