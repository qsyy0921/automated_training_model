package labelctl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type runtimeChat struct {
	cfg       Config
	in        io.Reader
	out       io.Writer
	errOut    io.Writer
	sessionID string
	turn      int
	noColor   bool
}

type runtimeStatusPayload struct {
	Runtime struct {
		Name         string               `json:"runtime"`
		ControlPlane string               `json:"control_plane"`
		AgentLoop    string               `json:"agent_loop"`
		Planner      runtimePlannerStatus `json:"planner"`
		Policy       string               `json:"policy"`
		EntryPoints  []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Transport string `json:"transport"`
			Status    string `json:"status"`
			Endpoint  string `json:"endpoint"`
		} `json:"entry_points"`
		ProviderRoutes []struct {
			ID       string `json:"id"`
			UseCase  string `json:"use_case"`
			Provider string `json:"provider"`
			Model    string `json:"model"`
		} `json:"provider_routes"`
		SubAgents []struct {
			ID         string   `json:"id"`
			Runtime    string   `json:"runtime"`
			ModelRoute string   `json:"model_route"`
			Status     string   `json:"status"`
			Capability []string `json:"capabilities"`
		} `json:"sub_agents"`
	} `json:"runtime"`
	Snapshot runtimeSnapshot `json:"snapshot"`
}

type runtimePlannerStatus struct {
	Mode          string `json:"mode"`
	MimoEnabled   bool   `json:"mimo_enabled"`
	MimoFallback  string `json:"mimo_fallback"`
	Python        string `json:"python"`
	PythonPath    string `json:"python_path"`
	TextModel     string `json:"text_model"`
	VisionModel   string `json:"vision_model"`
	TokenPresent  bool   `json:"token_present"`
	EffectiveMode string `json:"effective_mode"`
}

type runtimeSnapshot struct {
	StartedAt    string           `json:"started_at"`
	UpdatedAt    string           `json:"updated_at"`
	SessionCount int              `json:"session_count"`
	TraceCount   int              `json:"trace_count"`
	Sessions     []runtimeSession `json:"sessions"`
	RecentTraces []runtimeTrace   `json:"recent_traces"`
}

type runtimeSessionsPayload struct {
	Sessions []runtimeSession `json:"sessions"`
}

type runtimeTracesPayload struct {
	Traces []runtimeTrace `json:"traces"`
}

type runtimeJobsPayload struct {
	Jobs []runtimeModelJob `json:"jobs"`
}

type runtimeSession struct {
	Key         string   `json:"key"`
	AgentID     string   `json:"agent_id"`
	Channel     string   `json:"channel"`
	AccountID   string   `json:"account_id"`
	PeerKind    string   `json:"peer_kind"`
	PeerID      string   `json:"peer_id"`
	MessageCnt  int      `json:"message_count"`
	LastIntent  string   `json:"last_intent"`
	LastToolIDs []string `json:"last_tool_ids"`
	LastStatus  string   `json:"last_status"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type runtimeTrace struct {
	ID         string         `json:"id"`
	SessionKey string         `json:"session_key"`
	MessageID  string         `json:"message_id"`
	Channel    string         `json:"channel"`
	PeerKind   string         `json:"peer_kind"`
	PeerID     string         `json:"peer_id"`
	Intent     string         `json:"intent"`
	AgentID    string         `json:"agent_id"`
	ToolIDs    []string       `json:"tool_ids"`
	Status     string         `json:"status"`
	ReplyText  string         `json:"reply_text"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  string         `json:"created_at"`
}

type runtimeModelJob struct {
	ID        string `json:"id"`
	RepoID    string `json:"repo_id"`
	Status    string `json:"status"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func newRuntimeChat(cfg Config, in io.Reader, out io.Writer, errOut io.Writer) *runtimeChat {
	return &runtimeChat{
		cfg:       cfg,
		in:        in,
		out:       out,
		errOut:    errOut,
		sessionID: fmt.Sprintf("cli-%s", time.Now().Format("20060102-150405")),
		noColor:   os.Getenv("NO_COLOR") != "",
	}
}

func (c *runtimeChat) Run() error {
	c.printBanner()
	if err := c.printStartupSnapshot(); err != nil {
		fmt.Fprintf(c.errOut, "doctor: %v\n", err)
	}
	reader := bufio.NewReader(c.in)
	for {
		fmt.Fprintf(c.out, "\n%s ", c.prompt())
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		input := normalizeRuntimeInput(line)
		if input == "" {
			if errors.Is(err, io.EOF) {
				return nil
			}
			continue
		}
		if isExitCommand(input) {
			fmt.Fprintln(c.out, "bye")
			return nil
		}
		if handled, err := c.handleCommand(input); handled {
			if err != nil {
				fmt.Fprintf(c.errOut, "error: %v\n", err)
			}
			continue
		}
		c.turn++
		c.printUser(input)
		started := time.Now()
		reply, err := postRuntimeMessage(c.cfg, input)
		elapsed := time.Since(started)
		if err != nil {
			fmt.Fprintf(c.errOut, "error: %v\n", err)
			continue
		}
		trace, _ := c.latestTrace()
		c.printAssistant(reply.Reply.Text, trace, elapsed)
	}
}

func (c *runtimeChat) printBanner() {
	fmt.Fprintln(c.out, c.color("Automated Training Agent", "cyan"))
	fmt.Fprintf(c.out, "gateway: %s\n", c.cfg.addr)
	fmt.Fprintf(c.out, "session: %s  cwd: %s\n", c.sessionID, compactPath(mustGetwd()))
	fmt.Fprintln(c.out, "model routes: text=mimo-v2.5-pro  vision=mimo-v2.5")
	fmt.Fprintln(c.out, "type /help for commands, /exit to quit")
}

func (c *runtimeChat) printStartupSnapshot() error {
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "runtime: %s  planner=%s mimo=%t token=%t sessions=%d traces=%d\n", valueOr(status.Runtime.Name, "unknown"), valueOr(status.Runtime.Planner.EffectiveMode, "-"), status.Runtime.Planner.MimoEnabled, status.Runtime.Planner.TokenPresent, status.Snapshot.SessionCount, status.Snapshot.TraceCount)
	fmt.Fprintln(c.out, c.color("entry points", "bold"))
	for _, ep := range status.Runtime.EntryPoints {
		fmt.Fprintf(c.out, "  %-8s %-14s %-14s %s\n", ep.ID, ep.Status, ep.Transport, ep.Endpoint)
	}
	return nil
}

func (c *runtimeChat) prompt() string {
	return c.color(fmt.Sprintf("atm:%02d planner-agent>", c.turn+1), "green")
}

func (c *runtimeChat) handleCommand(input string) (bool, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return true, nil
	}
	command := strings.ToLower(parts[0])
	switch command {
	case "/help", "help":
		c.printHelp()
		return true, nil
	case "/status":
		return true, c.printStatus()
	case "/sessions":
		return true, c.printSessions()
	case "/traces":
		return true, c.printTraces()
	case "/jobs", "/tasks":
		return true, c.printJobs()
	case "/doctor":
		return true, c.printDoctor()
	case "/clear":
		fmt.Fprint(c.out, "\x1b[2J\x1b[H")
		c.printBanner()
		return true, nil
	case "/json":
		if len(parts) < 2 {
			return true, errors.New("usage: /json status|sessions|traces|jobs")
		}
		return true, c.printRawJSON(parts[1])
	case "/ping", "ping", "/bot-ping":
		c.turn++
		c.printUser("/bot-ping")
		started := time.Now()
		reply, err := postRuntimeMessage(c.cfg, "/bot-ping")
		if err != nil {
			return true, err
		}
		trace, _ := c.latestTrace()
		c.printAssistant(reply.Reply.Text, trace, time.Since(started))
		return true, nil
	default:
		return false, nil
	}
}

func (c *runtimeChat) printHelp() {
	fmt.Fprintln(c.out, c.color("commands", "bold"))
	fmt.Fprintln(c.out, "  /status      runtime, routes and current counters")
	fmt.Fprintln(c.out, "  /sessions    active channel/session table")
	fmt.Fprintln(c.out, "  /traces      recent agent/tool trace tree")
	fmt.Fprintln(c.out, "  /jobs        model/background job table")
	fmt.Fprintln(c.out, "  /doctor      server, runtime and local CLI diagnostics")
	fmt.Fprintln(c.out, "  /json <x>    raw JSON for status/sessions/traces/jobs")
	fmt.Fprintln(c.out, "  /clear       clear screen")
	fmt.Fprintln(c.out, "  /ping        send /bot-ping through the same runtime path")
	fmt.Fprintln(c.out, "  /exit        quit")
	fmt.Fprintln(c.out, "\nAny other text is sent to the Agent Runtime.")
}

func (c *runtimeChat) printStatus() error {
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err != nil {
		return err
	}
	fmt.Fprintln(c.out, c.color("runtime", "bold"))
	fmt.Fprintf(c.out, "  name: %s\n", valueOr(status.Runtime.Name, "unknown"))
	fmt.Fprintf(c.out, "  control: %s\n", valueOr(status.Runtime.ControlPlane, "unknown"))
	fmt.Fprintf(c.out, "  loop: %s\n", valueOr(status.Runtime.AgentLoop, "unknown"))
	fmt.Fprintf(c.out, "  planner: mode=%s effective=%s mimo=%t fallback=%s token=%t\n", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-"), status.Runtime.Planner.MimoEnabled, valueOr(status.Runtime.Planner.MimoFallback, "-"), status.Runtime.Planner.TokenPresent)
	fmt.Fprintf(c.out, "  planner python: %s\n", valueOr(status.Runtime.Planner.Python, "-"))
	fmt.Fprintf(c.out, "  planner pythonpath: %s\n", valueOr(status.Runtime.Planner.PythonPath, "-"))
	fmt.Fprintf(c.out, "  policy: %s\n", valueOr(status.Runtime.Policy, "unknown"))
	fmt.Fprintf(c.out, "  sessions=%d traces=%d updated=%s\n", status.Snapshot.SessionCount, status.Snapshot.TraceCount, compactTime(status.Snapshot.UpdatedAt))

	fmt.Fprintln(c.out, c.color("\nmodels", "bold"))
	for _, route := range status.Runtime.ProviderRoutes {
		fmt.Fprintf(c.out, "  %-18s %-7s %-16s %s\n", route.ID, route.Provider, route.Model, route.UseCase)
	}

	fmt.Fprintln(c.out, c.color("\nsub-agents", "bold"))
	for _, agent := range status.Runtime.SubAgents {
		fmt.Fprintf(c.out, "  %-22s %-10s %-16s %s\n", agent.ID, agent.Status, agent.ModelRoute, strings.Join(agent.Capability, ", "))
	}
	return nil
}

func (c *runtimeChat) printSessions() error {
	var payload runtimeSessionsPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/sessions", &payload); err != nil {
		return err
	}
	if len(payload.Sessions) == 0 {
		fmt.Fprintln(c.out, "no sessions")
		return nil
	}
	fmt.Fprintln(c.out, c.color("sessions", "bold"))
	for _, session := range payload.Sessions {
		fmt.Fprintf(c.out, "  %-18s %-10s %-10s messages=%d updated=%s\n", session.AgentID, session.LastStatus, session.LastIntent, session.MessageCnt, compactTime(session.UpdatedAt))
		fmt.Fprintf(c.out, "    %s:%s/%s  tools=%s\n", session.Channel, session.PeerKind, session.PeerID, joinOr(session.LastToolIDs, "-"))
	}
	return nil
}

func (c *runtimeChat) printTraces() error {
	var payload runtimeTracesPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/traces", &payload); err != nil {
		return err
	}
	if len(payload.Traces) == 0 {
		fmt.Fprintln(c.out, "no traces")
		return nil
	}
	fmt.Fprintln(c.out, c.color("recent traces", "bold"))
	limit := minInt(len(payload.Traces), 8)
	for i := 0; i < limit; i++ {
		trace := payload.Traces[i]
		c.printTraceLine(trace, i == limit-1)
	}
	return nil
}

func (c *runtimeChat) printJobs() error {
	var payload runtimeJobsPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/model-jobs", &payload); err != nil {
		return err
	}
	if len(payload.Jobs) == 0 {
		fmt.Fprintln(c.out, "no model/background jobs")
		return nil
	}
	fmt.Fprintln(c.out, c.color("model jobs", "bold"))
	for _, job := range payload.Jobs {
		fmt.Fprintf(c.out, "  %-24s %-12s %-10s %s\n", valueOr(job.ID, "-"), valueOr(job.Status, "-"), valueOr(job.Kind, "-"), valueOr(job.RepoID, "-"))
	}
	return nil
}

func (c *runtimeChat) printDoctor() error {
	fmt.Fprintln(c.out, c.color("doctor", "bold"))
	fmt.Fprintf(c.out, "  os: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(c.out, "  cwd: %s\n", mustGetwd())
	fmt.Fprintf(c.out, "  gateway: %s\n", c.cfg.addr)
	if err := checkHTTP(c.cfg.addr + "/healthz"); err != nil {
		fmt.Fprintf(c.out, "  healthz: failed (%v)\n", err)
	} else {
		fmt.Fprintln(c.out, "  healthz: ok")
	}
	if err := checkHTTP(c.cfg.addr + "/api/runtime/status"); err != nil {
		fmt.Fprintf(c.out, "  runtime: failed (%v)\n", err)
	} else {
		fmt.Fprintln(c.out, "  runtime: ok")
	}
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err == nil {
		fmt.Fprintf(c.out, "  planner: mode=%s effective=%s mimo=%t fallback=%s token=%t\n", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-"), status.Runtime.Planner.MimoEnabled, valueOr(status.Runtime.Planner.MimoFallback, "-"), status.Runtime.Planner.TokenPresent)
		fmt.Fprintf(c.out, "  planner pythonpath: %s\n", valueOr(status.Runtime.Planner.PythonPath, "-"))
	}
	fmt.Fprintf(c.out, "  LLM_BASE_URL: %s\n", presentOrMissing("LLM_BASE_URL"))
	fmt.Fprintf(c.out, "  LLM_MODEL: %s\n", presentOrMissing("LLM_MODEL"))
	fmt.Fprintf(c.out, "  LLM_API_KEY: %s\n", presentOrMissing("LLM_API_KEY"))
	fmt.Fprintf(c.out, "  ANTHROPIC_BASE_URL: %s\n", presentOrMissing("ANTHROPIC_BASE_URL"))
	fmt.Fprintf(c.out, "  ANTHROPIC_AUTH_TOKEN: %s\n", presentOrMissing("ANTHROPIC_AUTH_TOKEN"))
	return nil
}

func (c *runtimeChat) printRawJSON(name string) error {
	switch strings.ToLower(name) {
	case "status":
		return getJSON(c.cfg.addr + "/api/runtime/status")
	case "sessions":
		return getJSON(c.cfg.addr + "/api/runtime/sessions")
	case "traces":
		return getJSON(c.cfg.addr + "/api/runtime/traces")
	case "jobs", "model-jobs":
		return getJSON(c.cfg.addr + "/api/runtime/model-jobs")
	default:
		return fmt.Errorf("unknown json target: %s", name)
	}
}

func (c *runtimeChat) printUser(text string) {
	fmt.Fprintf(c.out, "\n%s %s\n", c.color("You", "blue"), text)
}

func (c *runtimeChat) printAssistant(text string, trace runtimeTrace, elapsed time.Duration) {
	title := trace.AgentID
	if title == "" {
		title = "planner-agent"
	}
	status := trace.Status
	if status == "" {
		status = "ok"
	}
	intent := trace.Intent
	if intent == "" {
		intent = "chat"
	}
	fmt.Fprintf(c.out, "\n%s %s · %s · %s\n", c.color("Agent", "cyan"), title, status, compactDuration(elapsed))
	fmt.Fprintf(c.out, "  intent: %s  tools: %s\n", intent, joinOr(trace.ToolIDs, "-"))
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		fmt.Fprintf(c.out, "  %s\n", line)
	}
	if len(trace.Metadata) > 0 {
		fmt.Fprintf(c.out, "  metadata: %s\n", compactMetadata(trace.Metadata))
	}
	if trace.SessionKey != "" {
		fmt.Fprintf(c.out, "  session: %s\n", trace.SessionKey)
	}
}

func (c *runtimeChat) printTraceLine(trace runtimeTrace, isLast bool) {
	tree := "├─"
	leaf := "│  ⎿"
	if isLast {
		tree = "└─"
		leaf = "   ⎿"
	}
	fmt.Fprintf(c.out, "  %s %s · %s · %s · tools=%s\n", tree, valueOr(trace.AgentID, "agent"), valueOr(trace.Status, "-"), valueOr(trace.Intent, "-"), joinOr(trace.ToolIDs, "-"))
	fmt.Fprintf(c.out, "  %s %s\n", leaf, firstLine(trace.ReplyText, 96))
}

func (c *runtimeChat) latestTrace() (runtimeTrace, error) {
	var payload runtimeTracesPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/traces", &payload); err != nil {
		return runtimeTrace{}, err
	}
	if len(payload.Traces) == 0 {
		return runtimeTrace{}, nil
	}
	return payload.Traces[0], nil
}

func (c *runtimeChat) color(text string, name string) string {
	if c.noColor {
		return text
	}
	codes := map[string]string{
		"bold":  "1",
		"blue":  "34",
		"cyan":  "36",
		"green": "32",
	}
	code := codes[name]
	if code == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func getJSONValue(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s", resp.Status, string(raw))
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse %s: %w", url, err)
	}
	return nil
}

func checkHTTP(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.New(resp.Status)
	}
	return nil
}

func isExitCommand(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "/exit", "exit", "/quit", "quit":
		return true
	default:
		return false
	}
}

func normalizeRuntimeInput(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\ufeff\u200b\u200c\u200d")
	return strings.TrimSpace(value)
}

func compactDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", value.Seconds())
}

func compactMetadata(metadata map[string]any) string {
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, minInt(len(keys), 4))
	for i, key := range keys {
		if i >= 4 {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, key+"="+firstLine(fmt.Sprint(metadata[key]), 48))
	}
	return strings.Join(parts, ", ")
}

func compactPath(value string) string {
	clean := filepath.Clean(value)
	parts := strings.Split(clean, string(os.PathSeparator))
	if len(parts) <= 3 {
		return clean
	}
	return strings.Join(parts[len(parts)-3:], string(os.PathSeparator))
}

func compactTime(value string) string {
	if value == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.Format("2006-01-02 15:04:05")
}

func firstLine(value string, maxLen int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r", ""))
	if value == "" {
		return "-"
	}
	value = strings.Split(value, "\n")[0]
	if len([]rune(value)) <= maxLen {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxLen-1]) + "..."
}

func joinOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, ",")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func presentOrMissing(name string) string {
	if strings.TrimSpace(os.Getenv(name)) == "" {
		return "missing"
	}
	return "set"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
