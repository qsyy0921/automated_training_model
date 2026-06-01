package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type OutboundConfig struct {
	Enabled     bool   `json:"enabled"`
	BaseURL     string `json:"base_url,omitempty"`
	AccessToken string `json:"-"`
	Timeout     time.Duration
}

type OutboundStatus struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url,omitempty"`
}

type HTTPOutboundSender struct {
	cfg    OutboundConfig
	client *http.Client
}

func OutboundConfigFromEnv() OutboundConfig {
	return OutboundConfig{
		Enabled:     parseBool(os.Getenv("QQ_ONEBOT_OUTBOUND_ENABLED")),
		BaseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("QQ_ONEBOT_HTTP_URL")), "/"),
		AccessToken: strings.TrimSpace(os.Getenv("QQ_ONEBOT_ACCESS_TOKEN")),
		Timeout:     10 * time.Second,
	}
}

func OutboundStatusFromEnv() OutboundStatus {
	cfg := OutboundConfigFromEnv()
	return OutboundStatus{Enabled: cfg.Enabled, BaseURL: cfg.BaseURL}
}

func NewHTTPOutboundSender(cfg OutboundConfig) *HTTPOutboundSender {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPOutboundSender{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *HTTPOutboundSender) Send(ctx context.Context, reply OneBotReply) error {
	if s == nil || !s.cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(s.cfg.BaseURL) == "" {
		return fmt.Errorf("QQ_ONEBOT_HTTP_URL is required when outbound is enabled")
	}
	action := strings.TrimSpace(reply.Action)
	if action == "" {
		return fmt.Errorf("onebot action is required")
	}
	endpoint, err := url.JoinPath(s.cfg.BaseURL, action)
	if err != nil {
		return err
	}
	body, err := json.Marshal(reply.Params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.AccessToken)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("onebot outbound %s returned %s", action, resp.Status)
	}
	return nil
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}
