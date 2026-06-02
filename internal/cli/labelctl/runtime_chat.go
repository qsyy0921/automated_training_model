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
	c.printPanel("Automated Training Agent", []string{
		"Gateway   " + c.cfg.addr,
		"Session   " + c.sessionID + "  cwd=" + compactPath(mustGetwd()),
		"Models    text=mimo-v2.5-pro  vision=mimo-v2.5",
		"Commands  /help  /status  /traces  /jobs  /doctor  /exit",
	}, "cyan")
}

func (c *runtimeChat) printStartupSnapshot() error {
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("Runtime   %s", valueOr(status.Runtime.Name, "unknown")),
		fmt.Sprintf("Planner   %s  mimo=%t  token=%t  fallback=%s", valueOr(status.Runtime.Planner.EffectiveMode, "-"), status.Runtime.Planner.MimoEnabled, status.Runtime.Planner.TokenPresent, valueOr(status.Runtime.Planner.MimoFallback, "-")),
		fmt.Sprintf("State     sessions=%d  traces=%d  updated=%s", status.Snapshot.SessionCount, status.Snapshot.TraceCount, compactTime(status.Snapshot.UpdatedAt)),
		"",
		"Entry Points",
	}
	for _, ep := range status.Runtime.EntryPoints {
		lines = append(lines, fmt.Sprintf("  %-8s %-14s %-14s %s", ep.ID, ep.Status, ep.Transport, ep.Endpoint))
	}
	c.printPanel("Runtime Snapshot", lines, "green")
	return nil
}

func (c *runtimeChat) prompt() string {
	return fmt.Sprintf("%s\n%s",
		c.color(fmt.Sprintf("╭─ atm:%02d", c.turn+1), "green")+" "+c.color("planner-agent", "cyan")+" "+c.color("mimo-v2.5-pro", "dim"),
		c.color("╰─›", "green"),
	)
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
	c.printPanel("Command Palette", []string{
		"/status      runtime, routes and current counters",
		"/sessions    active channel/session table",
		"/traces      recent agent/tool trace tree",
		"/jobs        model/background job table",
		"/doctor      server, runtime and local CLI diagnostics",
		"/json <x>    raw JSON for status/sessions/traces/jobs",
		"/clear       clear screen",
		"/ping        send /bot-ping through the same runtime path",
		"/exit        quit",
		"",
		"Any other text is sent to the Agent Runtime.",
	}, "cyan")
}

func (c *runtimeChat) printStatus() error {
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err != nil {
		return err
	}
	lines := []string{
		"Runtime",
		fmt.Sprintf("  name      %s", valueOr(status.Runtime.Name, "unknown")),
		fmt.Sprintf("  control   %s", valueOr(status.Runtime.ControlPlane, "unknown")),
		fmt.Sprintf("  loop      %s", valueOr(status.Runtime.AgentLoop, "unknown")),
		fmt.Sprintf("  policy    %s", valueOr(status.Runtime.Policy, "unknown")),
		fmt.Sprintf("  state     sessions=%d traces=%d updated=%s", status.Snapshot.SessionCount, status.Snapshot.TraceCount, compactTime(status.Snapshot.UpdatedAt)),
		"",
		"Planner",
		fmt.Sprintf("  mode      %s -> %s", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-")),
		fmt.Sprintf("  mimo      enabled=%t token=%t fallback=%s", status.Runtime.Planner.MimoEnabled, status.Runtime.Planner.TokenPresent, valueOr(status.Runtime.Planner.MimoFallback, "-")),
		fmt.Sprintf("  python    %s", valueOr(status.Runtime.Planner.Python, "-")),
		fmt.Sprintf("  path      %s", valueOr(status.Runtime.Planner.PythonPath, "-")),
		"",
		"Models",
	}
	for _, route := range status.Runtime.ProviderRoutes {
		lines = append(lines, fmt.Sprintf("  %-18s %-7s %-16s %s", route.ID, route.Provider, route.Model, route.UseCase))
	}
	lines = append(lines, "", "Sub-agents")
	for _, agent := range status.Runtime.SubAgents {
		lines = append(lines, fmt.Sprintf("  %-22s %-10s %-16s %s", agent.ID, agent.Status, agent.ModelRoute, strings.Join(agent.Capability, ", ")))
	}
	c.printPanel("Runtime Status", lines, "green")
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
	lines := []string{}
	for _, session := range payload.Sessions {
		lines = append(lines, fmt.Sprintf("%-18s %-10s %-10s messages=%d updated=%s", session.AgentID, session.LastStatus, session.LastIntent, session.MessageCnt, compactTime(session.UpdatedAt)))
		lines = append(lines, fmt.Sprintf("  %s:%s/%s  tools=%s", session.Channel, session.PeerKind, session.PeerID, joinOr(session.LastToolIDs, "-")))
	}
	c.printPanel("Sessions", lines, "cyan")
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
	lines := []string{}
	limit := minInt(len(payload.Traces), 8)
	for i := 0; i < limit; i++ {
		trace := payload.Traces[i]
		lines = append(lines, traceLines(trace, i == limit-1)...)
	}
	c.printPanel("Recent Traces", lines, "magenta")
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
	lines := []string{}
	for _, job := range payload.Jobs {
		lines = append(lines, fmt.Sprintf("%-24s %-12s %-10s %s", valueOr(job.ID, "-"), valueOr(job.Status, "-"), valueOr(job.Kind, "-"), valueOr(job.RepoID, "-")))
	}
	c.printPanel("Model Jobs", lines, "yellow")
	return nil
}

func (c *runtimeChat) printDoctor() error {
	lines := []string{
		fmt.Sprintf("os        %s/%s", runtime.GOOS, runtime.GOARCH),
		"cwd       " + mustGetwd(),
		"gateway   " + c.cfg.addr,
	}
	if err := checkHTTP(c.cfg.addr + "/healthz"); err != nil {
		lines = append(lines, fmt.Sprintf("healthz   failed (%v)", err))
	} else {
		lines = append(lines, "healthz   ok")
	}
	if err := checkHTTP(c.cfg.addr + "/api/runtime/status"); err != nil {
		lines = append(lines, fmt.Sprintf("runtime   failed (%v)", err))
	} else {
		lines = append(lines, "runtime   ok")
	}
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err == nil {
		lines = append(lines, fmt.Sprintf("planner   mode=%s effective=%s mimo=%t fallback=%s token=%t", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-"), status.Runtime.Planner.MimoEnabled, valueOr(status.Runtime.Planner.MimoFallback, "-"), status.Runtime.Planner.TokenPresent))
		lines = append(lines, fmt.Sprintf("python    %s", valueOr(status.Runtime.Planner.PythonPath, "-")))
	}
	lines = append(lines,
		"CLI env",
		"  LLM_BASE_URL          "+presentOrMissing("LLM_BASE_URL"),
		"  LLM_MODEL             "+presentOrMissing("LLM_MODEL"),
		"  LLM_API_KEY           "+presentOrMissing("LLM_API_KEY"),
		"  ANTHROPIC_BASE_URL    "+presentOrMissing("ANTHROPIC_BASE_URL"),
		"  ANTHROPIC_AUTH_TOKEN  "+presentOrMissing("ANTHROPIC_AUTH_TOKEN"),
	)
	c.printPanel("Doctor", lines, "yellow")
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
	c.printPanel("You", wrapText(text, c.contentWidth()), "blue")
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
	header := fmt.Sprintf("%s · %s · %s", title, status, compactDuration(elapsed))
	lines := []string{
		fmt.Sprintf("intent=%s  tools=%s", intent, joinOr(trace.ToolIDs, "-")),
		"",
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		lines = append(lines, wrapText(line, c.contentWidth())...)
	}
	if len(trace.Metadata) > 0 {
		lines = append(lines, "", "metadata  "+compactMetadata(trace.Metadata))
	}
	if trace.SessionKey != "" {
		lines = append(lines, "session   "+trace.SessionKey)
	}
	c.printPanel("Agent "+header, lines, statusColor(status))
}

func (c *runtimeChat) printTraceLine(trace runtimeTrace, isLast bool) {
	for _, line := range traceLines(trace, isLast) {
		fmt.Fprintln(c.out, line)
	}
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
		"bold":    "1",
		"blue":    "34",
		"cyan":    "36",
		"dim":     "2",
		"green":   "32",
		"magenta": "35",
		"red":     "31",
		"yellow":  "33",
	}
	code := codes[name]
	if code == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (c *runtimeChat) printPanel(title string, lines []string, color string) {
	width := c.panelWidth()
	titleText := " " + title + " "
	topLen := width - runeLen(titleText) - 3
	if topLen < 4 {
		topLen = 4
	}
	fmt.Fprintf(c.out, "\n%s\n", c.color("╭─"+titleText+strings.Repeat("─", topLen)+"╮", color))
	for _, line := range lines {
		if line == "" {
			fmt.Fprintf(c.out, "%s\n", c.color("│", "dim"))
			continue
		}
		for _, wrapped := range wrapText(line, c.contentWidth()) {
			fmt.Fprintf(c.out, "%s %s\n", c.color("│", "dim"), wrapped)
		}
	}
	fmt.Fprintf(c.out, "%s\n", c.color("╰"+strings.Repeat("─", width-2)+"╯", color))
}

func (c *runtimeChat) panelWidth() int {
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed >= 72 {
			return minInt(parsed, 110)
		}
	}
	return 96
}

func (c *runtimeChat) contentWidth() int {
	return c.panelWidth() - 4
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

func statusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "planned", "tool_planned", "tool_planned_with_guard":
		return "green"
	case "approval_required":
		return "yellow"
	case "failed", "tool_failed", "planning_failed", "preflight_failed":
		return "red"
	default:
		return "cyan"
	}
}

func traceLines(trace runtimeTrace, isLast bool) []string {
	tree := "├─"
	leaf := "│  ⎿"
	if isLast {
		tree = "└─"
		leaf = "   ⎿"
	}
	return []string{
		fmt.Sprintf("%s %s · %s · %s · tools=%s", tree, valueOr(trace.AgentID, "agent"), valueOr(trace.Status, "-"), valueOr(trace.Intent, "-"), joinOr(trace.ToolIDs, "-")),
		fmt.Sprintf("%s %s", leaf, firstLine(trace.ReplyText, 96)),
	}
}

func wrapText(value string, maxLen int) []string {
	value = strings.TrimRight(strings.ReplaceAll(value, "\r", ""), " \t")
	if strings.TrimSpace(value) == "" {
		return []string{""}
	}
	if maxLen < 20 {
		maxLen = 20
	}
	prefix := leadingWhitespace(value)
	content := strings.TrimSpace(value)
	available := maxLen - runeLen(prefix)
	if available < 20 {
		available = maxLen
		prefix = ""
	}
	runes := []rune(content)
	if runeLen(prefix)+len(runes) <= maxLen {
		return []string{value}
	}
	lines := []string{}
	continuationPrefix := prefix
	if prefix != "" {
		continuationPrefix = prefix + "  "
	}
	for len(runes) > available {
		cut := available
		for i := available; i > available/2; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				cut = i
				break
			}
		}
		linePrefix := prefix
		if len(lines) > 0 {
			linePrefix = continuationPrefix
		}
		lines = append(lines, linePrefix+strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}
	if len(runes) > 0 {
		linePrefix := prefix
		if len(lines) > 0 {
			linePrefix = continuationPrefix
		}
		lines = append(lines, linePrefix+string(runes))
	}
	return lines
}

func compactPath(value string) string {
	clean := filepath.Clean(value)
	parts := strings.Split(clean, string(os.PathSeparator))
	if len(parts) <= 3 {
		return clean
	}
	return strings.Join(parts[len(parts)-3:], string(os.PathSeparator))
}

func runeLen(value string) int {
	return len([]rune(value))
}

func leadingWhitespace(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
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
