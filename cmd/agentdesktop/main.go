package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type config struct {
	addr  string
	token string
}

type endpoint struct {
	method string
	path   string
	body   []byte
}

type runtimeStatus struct {
	Desktop struct {
		Status  string `json:"status"`
		Profile string `json:"profile"`
		Gateway string `json:"gateway"`
		Runtime string `json:"runtime"`
	} `json:"desktop"`
}

type sessionList struct {
	Sessions []struct {
		Key          string   `json:"key"`
		AgentID      string   `json:"agent_id"`
		Channel      string   `json:"channel"`
		PeerKind     string   `json:"peer_kind"`
		PeerID       string   `json:"peer_id"`
		LastIntent   string   `json:"last_intent"`
		LastStatus   string   `json:"last_status"`
		LastToolIDs  []string `json:"last_tool_ids"`
		MessageCount int      `json:"message_count"`
	} `json:"sessions"`
}

type traceList struct {
	Traces []struct {
		ID        string   `json:"id"`
		AgentID   string   `json:"agent_id"`
		Intent    string   `json:"intent"`
		Status    string   `json:"status"`
		ToolIDs   []string `json:"tool_ids"`
		ReplyText string   `json:"reply_text"`
		Error     string   `json:"error"`
	} `json:"traces"`
}

type jobList struct {
	Jobs []struct {
		ID              string `json:"id"`
		Kind            string `json:"kind"`
		RepoID          string `json:"repo_id"`
		Status          string `json:"status"`
		Message         string `json:"message"`
		Error           string `json:"error"`
		ProgressPercent int    `json:"progress_percent"`
		Resumable       bool   `json:"resumable"`
		CancelRequested bool   `json:"cancel_requested"`
	} `json:"jobs"`
}

type sendResponse struct {
	Reply struct {
		Text string `json:"text"`
	} `json:"reply"`
}

func main() {
	cfg := config{token: firstEnv("ATM_GATEWAY_TOKEN", "GATEWAY_AUTH_TOKEN")}
	flag.StringVar(&cfg.addr, "addr", "http://127.0.0.1:7870", "labelserver base URL")
	flag.StringVar(&cfg.token, "token", cfg.token, "Gateway bearer token")
	flag.Parse()

	if err := run(cfg, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cfg config, args []string) error {
	if len(args) == 0 {
		return renderDesktopDashboard(cfg)
	}
	switch args[0] {
	case "status", "dashboard":
		return renderDesktopDashboard(cfg)
	case "json":
		raw, err := request(cfg, endpoint{method: http.MethodGet, path: "/api/desktop/status"})
		if err != nil {
			return err
		}
		return printJSON(raw)
	case "sessions":
		return renderSessions(cfg)
	case "traces":
		return renderTraces(cfg)
	case "jobs", "model-jobs":
		return renderJobs(cfg)
	case "send":
		text := strings.TrimSpace(strings.Join(args[1:], " "))
		if text == "" {
			return errors.New("usage: agentdesktop send <message>")
		}
		return sendRuntimeMessage(cfg, text)
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown desktop command: %s", args[0])
	}
}

func renderDesktopDashboard(cfg config) error {
	raw, err := request(cfg, endpoint{method: http.MethodGet, path: "/api/desktop/status"})
	if err != nil {
		return err
	}
	var status runtimeStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return printJSON(raw)
	}
	fmt.Println("Automated Training Desktop")
	fmt.Printf("gateway  %s\n", status.Desktop.Gateway)
	fmt.Printf("profile  %s\n", status.Desktop.Profile)
	fmt.Printf("runtime  %s\n", status.Desktop.Runtime)
	fmt.Printf("status   %s\n", status.Desktop.Status)
	fmt.Println()
	fmt.Println("commands")
	fmt.Println("  status      desktop runtime dashboard")
	fmt.Println("  sessions    runtime sessions")
	fmt.Println("  traces      recent runtime traces")
	fmt.Println("  jobs        model jobs")
	fmt.Println("  send <text> send through QQ test adapter into Agent Runtime")
	fmt.Println("  json        raw /api/desktop/status JSON")
	return nil
}

func renderSessions(cfg config) error {
	raw, err := request(cfg, endpoint{method: http.MethodGet, path: "/api/runtime/sessions"})
	if err != nil {
		return err
	}
	var payload sessionList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return printJSON(raw)
	}
	fmt.Println("Runtime Sessions")
	if len(payload.Sessions) == 0 {
		fmt.Println("  no sessions")
		return nil
	}
	for _, session := range payload.Sessions {
		fmt.Printf("  %s  %s/%s:%s  %s %s  messages=%d\n", session.AgentID, session.Channel, session.PeerKind, session.PeerID, session.LastIntent, session.LastStatus, session.MessageCount)
		if len(session.LastToolIDs) > 0 {
			fmt.Printf("    tools=%s\n", strings.Join(session.LastToolIDs, ","))
		}
		fmt.Printf("    %s\n", session.Key)
	}
	return nil
}

func renderTraces(cfg config) error {
	raw, err := request(cfg, endpoint{method: http.MethodGet, path: "/api/runtime/traces?limit=8"})
	if err != nil {
		return err
	}
	var payload traceList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return printJSON(raw)
	}
	fmt.Println("Runtime Traces")
	if len(payload.Traces) == 0 {
		fmt.Println("  no traces")
		return nil
	}
	for _, trace := range payload.Traces {
		fmt.Printf("  %s  %s  %s  tools=%s\n", trace.AgentID, trace.Intent, trace.Status, joinOrDash(trace.ToolIDs))
		if trace.Error != "" {
			fmt.Printf("    error=%s\n", trace.Error)
		} else if trace.ReplyText != "" {
			fmt.Printf("    %s\n", singleLine(trace.ReplyText, 120))
		}
	}
	return nil
}

func renderJobs(cfg config) error {
	raw, err := request(cfg, endpoint{method: http.MethodGet, path: "/api/runtime/model-jobs"})
	if err != nil {
		return err
	}
	var payload jobList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return printJSON(raw)
	}
	fmt.Println("Model Jobs")
	if len(payload.Jobs) == 0 {
		fmt.Println("  no model jobs")
		return nil
	}
	for _, job := range payload.Jobs {
		fmt.Printf("  %s  %s  %s  %d%%\n", job.ID, job.RepoID, job.Status, job.ProgressPercent)
		message := job.Message
		if job.Error != "" {
			message = job.Error
		}
		flags := []string{}
		if job.Resumable {
			flags = append(flags, "resumable")
		}
		if job.CancelRequested {
			flags = append(flags, "cancel requested")
		}
		if message != "" || len(flags) > 0 {
			fmt.Printf("    %s %s\n", message, strings.Join(flags, " "))
		}
	}
	return nil
}

func sendRuntimeMessage(cfg config, text string) error {
	raw, err := json.Marshal(map[string]any{
		"id":         fmt.Sprintf("desktop_%d", time.Now().UnixNano()),
		"channel":    "qq",
		"account_id": "default",
		"peer": map[string]any{
			"channel":    "qq",
			"account_id": "default",
			"kind":       "direct",
			"id":         "desktop-runtime",
		},
		"sender_id": "desktop-runtime",
		"text":      text,
	})
	if err != nil {
		return err
	}
	out, err := request(cfg, endpoint{method: http.MethodPost, path: "/api/channels/qq/test-message", body: raw})
	if err != nil {
		return err
	}
	var payload sendResponse
	if err := json.Unmarshal(out, &payload); err != nil {
		return printJSON(out)
	}
	fmt.Println(payload.Reply.Text)
	return nil
}

func request(cfg config, ep endpoint) ([]byte, error) {
	method := ep.method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequest(method, strings.TrimRight(cfg.addr, "/")+ep.path, bytes.NewReader(ep.body))
	if err != nil {
		return nil, err
	}
	if ep.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(cfg.token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.token))
		req.Header.Set("X-Gateway-Token", strings.TrimSpace(cfg.token))
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(raw))
	}
	return raw, nil
}

func printJSON(raw []byte) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func singleLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 && len(value) > limit {
		return value[:limit-3] + "..."
	}
	return value
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func usage() {
	fmt.Println("usage: agentdesktop [-addr URL] [-token TOKEN] [status|sessions|traces|jobs|send <message>|json]")
}
