package labelctl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Gateway  struct {
		Auth gatewayAuthStatus `json:"auth"`
	} `json:"gateway"`
}

type gatewayAuthStatus struct {
	TokenConfigured          bool     `json:"token_configured"`
	RemoteRequiresToken      bool     `json:"remote_requires_token"`
	LoopbackBypass           bool     `json:"loopback_bypass"`
	RequireTokenForLoopback  bool     `json:"require_token_for_loopback"`
	AllowRemoteWithoutToken  bool     `json:"allow_remote_without_token"`
	AllowedOriginsConfigured bool     `json:"allowed_origins_configured"`
	AllowedOrigins           []string `json:"allowed_origins"`
}

type runtimePlannerStatus struct {
	Mode          string `json:"mode"`
	MimoEnabled   bool   `json:"mimo_enabled"`
	MimoFallback  string `json:"mimo_fallback"`
	Python        string `json:"python"`
	PythonPath    string `json:"python_path"`
	Transport     string `json:"transport"`
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

type runtimeTasksPayload struct {
	Tasks []runtimeTask `json:"tasks"`
}

type runtimeJobPayload struct {
	Job runtimeModelJob `json:"job"`
}

type runtimeTaskPayload struct {
	Task runtimeTask `json:"task"`
}

type runtimeJobLineagePayload struct {
	JobID   string            `json:"job_id"`
	RootID  string            `json:"root_id"`
	Count   int               `json:"count"`
	Lineage []runtimeModelJob `json:"lineage"`
}

type runtimeTaskLineagePayload struct {
	TaskID  string        `json:"task_id"`
	RootID  string        `json:"root_id"`
	Count   int           `json:"count"`
	Lineage []runtimeTask `json:"lineage"`
}

type runtimeModelJobLog struct {
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type runtimeTaskLog struct {
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type runtimeModelJobLogsPayload struct {
	JobID           string               `json:"job_id"`
	Status          string               `json:"status"`
	ProgressPercent int                  `json:"progress_percent"`
	Retryable       bool                 `json:"retryable"`
	Attempt         int                  `json:"attempt"`
	MaxAttempts     int                  `json:"max_attempts"`
	WorkerHeartbeat *runtimeJobHeartbeat `json:"worker_heartbeat"`
	Artifacts       []runtimeJobArtifact `json:"artifacts"`
	Stdout          string               `json:"stdout"`
	Stderr          string               `json:"stderr"`
	Metadata        map[string]any       `json:"metadata"`
	Logs            []runtimeModelJobLog `json:"logs"`
}

type runtimeTaskLogsPayload struct {
	TaskID          string               `json:"task_id"`
	Type            string               `json:"type"`
	Status          string               `json:"status"`
	ProgressPercent int                  `json:"progress_percent"`
	Message         string               `json:"message"`
	Retryable       bool                 `json:"retryable"`
	Attempt         int                  `json:"attempt"`
	MaxAttempts     int                  `json:"max_attempts"`
	WorkerHeartbeat *runtimeJobHeartbeat `json:"worker_heartbeat"`
	Artifacts       []runtimeJobArtifact `json:"artifacts"`
	Stdout          string               `json:"stdout"`
	Stderr          string               `json:"stderr"`
	Metadata        map[string]any       `json:"metadata"`
	Logs            []runtimeTaskLog     `json:"logs"`
}

type runtimeArtifactManifestPayload struct {
	Path     string                  `json:"path"`
	Manifest runtimeArtifactManifest `json:"manifest"`
}

type runtimeArtifactManifest struct {
	SchemaVersion   string                 `json:"schema_version"`
	ArtifactSummary runtimeArtifactSummary `json:"artifact_summary"`
	Artifacts       []runtimeJobArtifact   `json:"artifacts"`
	Metadata        map[string]string      `json:"metadata"`
}

type runtimeArtifactSummary struct {
	ArtifactCount       int                             `json:"artifact_count"`
	RoleCounts          map[string]int                  `json:"role_counts"`
	KindCounts          map[string]int                  `json:"kind_counts"`
	ExecutionModeCounts map[string]int                  `json:"execution_mode_counts"`
	PrimaryArtifact     *runtimeArtifactPrimaryArtifact `json:"primary_artifact"`
}

type runtimeArtifactPrimaryArtifact struct {
	Name          string `json:"name"`
	URI           string `json:"uri"`
	Kind          string `json:"kind"`
	Role          string `json:"role"`
	ExecutionMode string `json:"execution_mode"`
}

type runtimeJobHeartbeat struct {
	At      string `json:"at"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type runtimeJobArtifact struct {
	Name     string            `json:"name"`
	URI      string            `json:"uri"`
	Kind     string            `json:"kind"`
	Metadata map[string]string `json:"metadata"`
}

type runtimeJobStreamEvent struct {
	Type            string               `json:"type"`
	JobID           string               `json:"job_id"`
	Status          string               `json:"status"`
	ProgressPercent int                  `json:"progress_percent"`
	Message         string               `json:"message"`
	Retryable       bool                 `json:"retryable"`
	Attempt         int                  `json:"attempt"`
	MaxAttempts     int                  `json:"max_attempts"`
	WorkerHeartbeat *runtimeJobHeartbeat `json:"worker_heartbeat"`
	Artifacts       []runtimeJobArtifact `json:"artifacts"`
	Stdout          string               `json:"stdout"`
	Stderr          string               `json:"stderr"`
	Metadata        map[string]any       `json:"metadata"`
	Log             runtimeModelJobLog   `json:"log"`
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
	ID              string `json:"id"`
	ParentID        string `json:"parent_id"`
	RepoID          string `json:"repo_id"`
	Status          string `json:"status"`
	Kind            string `json:"kind"`
	Message         string `json:"message"`
	ProgressPercent int    `json:"progress_percent"`
	CancelRequested bool   `json:"cancel_requested"`
	Resumable       bool   `json:"resumable"`
	Retryable       bool   `json:"retryable"`
	Attempt         int    `json:"attempt"`
	MaxAttempts     int    `json:"max_attempts"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type runtimeTask struct {
	ID              string `json:"id"`
	ParentID        string `json:"parent_id"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	Error           string `json:"error"`
	ProgressPercent int    `json:"progress_percent"`
	Resumable       bool   `json:"resumable"`
	Retryable       bool   `json:"retryable"`
	Attempt         int    `json:"attempt"`
	MaxAttempts     int    `json:"max_attempts"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type runtimeStreamEvent struct {
	Type          string                `json:"type"`
	Delta         string                `json:"delta,omitempty"`
	Text          string                `json:"text,omitempty"`
	Status        string                `json:"status,omitempty"`
	Message       string                `json:"message,omitempty"`
	Intent        string                `json:"intent,omitempty"`
	AgentID       string                `json:"agent_id,omitempty"`
	ToolIDs       []string              `json:"tool_ids,omitempty"`
	ToolID        string                `json:"tool_id,omitempty"`
	Session       string                `json:"session,omitempty"`
	ElapsedMS     int64                 `json:"elapsed_ms,omitempty"`
	ErrorEnvelope *runtimeErrorEnvelope `json:"error_envelope,omitempty"`
}

type runtimeTaskStreamEvent struct {
	Type            string               `json:"type"`
	TaskID          string               `json:"task_id"`
	Status          string               `json:"status"`
	ProgressPercent int                  `json:"progress_percent"`
	Message         string               `json:"message"`
	Retryable       bool                 `json:"retryable"`
	Attempt         int                  `json:"attempt"`
	MaxAttempts     int                  `json:"max_attempts"`
	WorkerHeartbeat *runtimeJobHeartbeat `json:"worker_heartbeat"`
	Artifacts       []runtimeJobArtifact `json:"artifacts"`
	Stdout          string               `json:"stdout"`
	Stderr          string               `json:"stderr"`
	Metadata        map[string]any       `json:"metadata"`
	Log             runtimeTaskLog       `json:"log"`
}

type runtimeErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Retryable bool   `json:"retryable"`
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
		reply, elapsed, streamed, err := c.postRuntimeMessageWithProgress(input)
		if err != nil {
			fmt.Fprintf(c.errOut, "error: %v\n", err)
			continue
		}
		if streamed {
			continue
		}
		trace, _ := c.latestTrace()
		if reply.Reply.Text != "" {
			c.printAssistant(reply.Reply.Text, trace, elapsed)
		} else {
			c.printAssistantFooter(trace, elapsed)
		}
	}
}

func (c *runtimeChat) printBanner() {
	c.printPanel("Automated Training Agent", []string{
		"Gateway   " + c.cfg.addr,
		"Session   " + c.sessionID + "  cwd=" + compactPath(mustGetwd()),
		"Models    text=mimo-v2.5-pro  vision=mimo-v2.5",
		"Commands  /help  /status  /traces  /jobs  /tasks  /doctor  /exit",
	}, "cyan")
}

func (c *runtimeChat) printStartupSnapshot() error {
	var status runtimeStatusPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/status", &status); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("Runtime   %s", valueOr(status.Runtime.Name, "unknown")),
		fmt.Sprintf("Planner   %s  transport=%s  mimo=%t  token=%t  fallback=%s", valueOr(status.Runtime.Planner.EffectiveMode, "-"), valueOr(status.Runtime.Planner.Transport, "-"), status.Runtime.Planner.MimoEnabled, status.Runtime.Planner.TokenPresent, valueOr(status.Runtime.Planner.MimoFallback, "-")),
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
	case "/jobs":
		return true, c.printJobs()
	case "/tasks":
		return true, c.printTasks()
	case "/job":
		if len(parts) < 2 {
			return true, errors.New("usage: /job <job_id>")
		}
		return true, c.printJob(parts[1])
	case "/task":
		if len(parts) < 2 {
			return true, errors.New("usage: /task <task_id>")
		}
		return true, c.printTask(parts[1])
	case "/job-logs", "/logs-job":
		if len(parts) < 2 {
			return true, errors.New("usage: /job-logs <job_id>")
		}
		return true, c.printJobLogs(parts[1])
	case "/job-manifest":
		if len(parts) < 2 {
			return true, errors.New("usage: /job-manifest <job_id>")
		}
		return true, c.printJobManifest(parts[1])
	case "/job-lineage":
		if len(parts) < 2 {
			return true, errors.New("usage: /job-lineage <job_id>")
		}
		return true, c.printJobLineage(parts[1])
	case "/task-logs", "/logs-task":
		if len(parts) < 2 {
			return true, errors.New("usage: /task-logs <task_id>")
		}
		return true, c.printTaskLogs(parts[1])
	case "/task-manifest":
		if len(parts) < 2 {
			return true, errors.New("usage: /task-manifest <task_id>")
		}
		return true, c.printTaskManifest(parts[1])
	case "/task-lineage":
		if len(parts) < 2 {
			return true, errors.New("usage: /task-lineage <task_id>")
		}
		return true, c.printTaskLineage(parts[1])
	case "/follow-job":
		if len(parts) < 2 {
			return true, errors.New("usage: /follow-job <job_id>")
		}
		return true, c.followJob(parts[1])
	case "/follow-task":
		if len(parts) < 2 {
			return true, errors.New("usage: /follow-task <task_id>")
		}
		return true, c.followTask(parts[1])
	case "/resume-task":
		if len(parts) < 2 {
			return true, errors.New("usage: /resume-task <task_id>")
		}
		return true, c.resumeTask(parts[1])
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
		reply, elapsed, streamed, err := c.postRuntimeMessageWithProgress("/bot-ping")
		if err != nil {
			return true, err
		}
		if streamed {
			return true, nil
		}
		trace, _ := c.latestTrace()
		if reply.Reply.Text != "" {
			c.printAssistant(reply.Reply.Text, trace, elapsed)
		} else {
			c.printAssistantFooter(trace, elapsed)
		}
		return true, nil
	default:
		return false, nil
	}
}

func (c *runtimeChat) postRuntimeMessageWithProgress(input string) (runtimeSendResponse, time.Duration, bool, error) {
	started := time.Now()
	if !cliStreamDisabled() {
		if reply, elapsed, printed, err := c.postRuntimeMessageStream(input, started); err == nil {
			if printed {
				return runtimeSendResponse{}, elapsed, true, nil
			}
			return reply, elapsed, false, nil
		}
	}
	type result struct {
		reply runtimeSendResponse
		err   error
	}
	done := make(chan result, 1)
	go func() {
		reply, err := postRuntimeMessage(c.cfg, input)
		done <- result{reply: reply, err: err}
	}()

	ticker := time.NewTicker(1200 * time.Millisecond)
	defer ticker.Stop()
	rendered := false
	for {
		select {
		case res := <-done:
			if rendered {
				fmt.Fprint(c.out, "\r\x1b[2K")
			}
			return res.reply, time.Since(started), false, res.err
		case <-ticker.C:
			rendered = true
			fmt.Fprintf(c.out, "\r%s", c.color(fmt.Sprintf("planner-agent working... %.1fs", time.Since(started).Seconds()), "dim"))
		}
	}
}

func (c *runtimeChat) postRuntimeMessageStream(input string, started time.Time) (runtimeSendResponse, time.Duration, bool, error) {
	raw, err := runtimeInboundMessageBody(input, "cli-runtime")
	if err != nil {
		return runtimeSendResponse{}, 0, false, err
	}
	req, err := http.NewRequest(http.MethodPost, c.cfg.addr+"/api/runtime/stream-message", bytes.NewReader(raw))
	if err != nil {
		return runtimeSendResponse{}, 0, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyGatewayAuth(req, c.cfg.token)
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return runtimeSendResponse{}, 0, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return runtimeSendResponse{}, 0, false, fmt.Errorf("%s: %s", resp.Status, string(bodyBytes))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	var full strings.Builder
	var final runtimeStreamEvent
	var startedPanel bool
	var statusRendered bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event runtimeStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return runtimeSendResponse{}, 0, startedPanel, fmt.Errorf("parse runtime stream event: %w: %s", err, line)
		}
		switch event.Type {
		case "start":
			if event.Intent != "" {
				final.Intent = event.Intent
			}
			if event.AgentID != "" {
				final.AgentID = event.AgentID
			}
		case "status", "tool_start":
			statusRendered = true
			message := valueOr(event.Message, "runtime working")
			fmt.Fprintf(c.out, "\r%s", c.color(fmt.Sprintf("%s %.1fs", message, time.Since(started).Seconds()), "dim"))
		case "tool_progress":
			if statusRendered && !startedPanel {
				fmt.Fprint(c.out, "\r\x1b[2K")
			}
			statusRendered = false
			fmt.Fprintf(c.out, "\n%s\n", c.color(toolProgressLine(event), "dim"))
		case "delta":
			if event.Delta == "" {
				continue
			}
			if !startedPanel {
				if statusRendered {
					fmt.Fprint(c.out, "\r\x1b[2K")
				}
				c.printStreamHeader(final.AgentID, final.Intent)
				startedPanel = true
			}
			fmt.Fprint(c.out, event.Delta)
			full.WriteString(event.Delta)
		case "final":
			final = mergeRuntimeStreamFinal(final, event)
			if full.Len() == 0 && event.Text != "" {
				full.WriteString(event.Text)
			}
		case "error":
			if statusRendered && !startedPanel {
				fmt.Fprint(c.out, "\r\x1b[2K")
			}
			return runtimeSendResponse{}, 0, startedPanel, errors.New(runtimeStreamErrorMessage(event))
		}
	}
	if err := scanner.Err(); err != nil {
		return runtimeSendResponse{}, 0, startedPanel, err
	}
	elapsed := time.Since(started)
	if startedPanel {
		c.printStreamFooter(final, elapsed)
		return runtimeSendResponse{}, elapsed, true, nil
	}
	if statusRendered {
		fmt.Fprint(c.out, "\r\x1b[2K")
	}
	var reply runtimeSendResponse
	reply.Reply.Text = strings.TrimSpace(full.String())
	return reply, elapsed, false, nil
}

func (c *runtimeChat) printStreamHeader(agentID string, intent string) {
	if agentID == "" {
		agentID = "planner-agent"
	}
	if intent == "" {
		intent = "chat"
	}
	fmt.Fprintf(c.out, "\n%s\n", c.color(fmt.Sprintf("╭─ Agent %s · streaming · intent=%s", agentID, intent), "green"))
	fmt.Fprintf(c.out, "%s\n", c.color("│", "dim"))
}

func (c *runtimeChat) printStreamFooter(event runtimeStreamEvent, elapsed time.Duration) {
	status := valueOr(event.Status, "ok")
	intent := valueOr(event.Intent, "chat")
	tools := joinOr(event.ToolIDs, "-")
	session := event.Session
	if session == "" {
		session = "-"
	}
	fmt.Fprintf(c.out, "\n%s %s\n", c.color("│", "dim"), c.color(fmt.Sprintf("status=%s  intent=%s  tools=%s  elapsed=%s", status, intent, tools, compactDuration(elapsed)), "dim"))
	fmt.Fprintf(c.out, "%s %s\n", c.color("│", "dim"), c.color("session "+session, "dim"))
	fmt.Fprintf(c.out, "%s\n", c.color("╰"+strings.Repeat("─", c.panelWidth()-2)+"╯", statusColor(status)))
}

func (c *runtimeChat) printAssistantFooter(trace runtimeTrace, elapsed time.Duration) {
	status := valueOr(trace.Status, "ok")
	intent := valueOr(string(trace.Intent), "chat")
	agentID := valueOr(trace.AgentID, "planner-agent")
	c.printPanel("Agent "+agentID+" · "+status+" · "+compactDuration(elapsed), []string{
		fmt.Sprintf("intent=%s  tools=%s", intent, joinOr(trace.ToolIDs, "-")),
		"session   " + valueOr(trace.SessionKey, "-"),
	}, statusColor(status))
}

func mergeRuntimeStreamFinal(base runtimeStreamEvent, next runtimeStreamEvent) runtimeStreamEvent {
	if next.Type != "" {
		base.Type = next.Type
	}
	if next.Text != "" {
		base.Text = next.Text
	}
	if next.Status != "" {
		base.Status = next.Status
	}
	if next.Message != "" {
		base.Message = next.Message
	}
	if next.Intent != "" {
		base.Intent = next.Intent
	}
	if next.AgentID != "" {
		base.AgentID = next.AgentID
	}
	if len(next.ToolIDs) > 0 {
		base.ToolIDs = next.ToolIDs
	}
	if next.Session != "" {
		base.Session = next.Session
	}
	if next.ElapsedMS > 0 {
		base.ElapsedMS = next.ElapsedMS
	}
	return base
}

func toolProgressLine(event runtimeStreamEvent) string {
	message := valueOr(event.Message, "tool progress")
	tool := event.ToolID
	if tool == "" {
		tool = joinOr(event.ToolIDs, "-")
	}
	status := valueOr(event.Status, "running")
	return fmt.Sprintf("  • tool=%s status=%s %s", tool, status, message)
}

func runtimeStreamErrorMessage(event runtimeStreamEvent) string {
	if event.ErrorEnvelope != nil && strings.TrimSpace(event.ErrorEnvelope.Message) != "" {
		return event.ErrorEnvelope.Message
	}
	return valueOr(event.Message, "runtime stream failed")
}

func modelJobLogLine(log runtimeModelJobLog) string {
	return fmt.Sprintf("%s  %-5s  %s", compactTime(log.At), valueOr(log.Level, "info"), valueOr(log.Message, "-"))
}

func cliStreamDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_CLI_STREAM"))) {
	case "0", "false", "no", "off", "none":
		return true
	default:
		return false
	}
}

func (c *runtimeChat) printHelp() {
	c.printPanel("Command Palette", []string{
		"/status      runtime, routes and current counters",
		"/sessions    active channel/session table",
		"/traces      recent agent/tool trace tree",
		"/jobs        model/background job table",
		"/tasks       lifecycle task table",
		"/job <id>    model/background job detail",
		"/task <id>   lifecycle task detail",
		"/job-logs <id>    model job lifecycle logs",
		"/job-manifest <id> model job artifact manifest",
		"/job-lineage <id> model job resume lineage",
		"/task-logs <id>   lifecycle task logs",
		"/task-manifest <id> lifecycle task artifact manifest",
		"/task-lineage <id> lifecycle task resume lineage",
		"/follow-job <id>  stream model job logs until terminal or timeout",
		"/follow-task <id> stream lifecycle task logs until terminal or timeout",
		"/resume-task <id> requeue interrupted/failed lifecycle task",
		"/doctor      server, runtime and local CLI diagnostics",
		"/json <x>    raw JSON for status/sessions/traces/jobs/tasks",
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
		fmt.Sprintf("  auth      token=%t remote_requires_token=%t loopback_bypass=%t origins=%s", status.Gateway.Auth.TokenConfigured, status.Gateway.Auth.RemoteRequiresToken, status.Gateway.Auth.LoopbackBypass, joinOr(status.Gateway.Auth.AllowedOrigins, "-")),
		"",
		"Planner",
		fmt.Sprintf("  mode      %s -> %s", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-")),
		fmt.Sprintf("  transport %s", valueOr(status.Runtime.Planner.Transport, "-")),
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
		flags := []string{}
		if job.CancelRequested {
			flags = append(flags, "cancel")
		}
		if job.Resumable {
			flags = append(flags, "resumable")
		}
		line := fmt.Sprintf("%-24s %-12s %3d%% %-10s %s", valueOr(job.ID, "-"), valueOr(job.Status, "-"), job.ProgressPercent, valueOr(job.Kind, "-"), valueOr(job.RepoID, "-"))
		if job.Message != "" {
			line += "  " + job.Message
		}
		if len(flags) > 0 {
			line += "  [" + strings.Join(flags, ",") + "]"
		}
		lines = append(lines, line)
	}
	c.printPanel("Model Jobs", lines, "yellow")
	return nil
}

func (c *runtimeChat) printTasks() error {
	var payload runtimeTasksPayload
	if err := getJSONValue(c.cfg.addr+"/api/tasks", &payload); err != nil {
		return err
	}
	if len(payload.Tasks) == 0 {
		fmt.Fprintln(c.out, "no lifecycle tasks")
		return nil
	}
	lines := []string{}
	for _, task := range payload.Tasks {
		line := fmt.Sprintf("%-24s %-12s %3d%% %-18s", valueOr(task.ID, "-"), valueOr(task.Status, "-"), task.ProgressPercent, valueOr(task.Type, "-"))
		if task.Message != "" {
			line += "  " + task.Message
		}
		if task.Resumable {
			line += "  [resumable]"
		}
		lines = append(lines, line)
	}
	c.printPanel("Lifecycle Tasks", lines, "yellow")
	return nil
}

func (c *runtimeChat) printJob(id string) error {
	var payload runtimeJobPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(id), &payload); err != nil {
		return err
	}
	job := payload.Job
	lines := []string{
		fmt.Sprintf("id        %s", valueOr(job.ID, "-")),
		fmt.Sprintf("repo      %s", valueOr(job.RepoID, "-")),
		fmt.Sprintf("kind      %s", valueOr(job.Kind, "-")),
		fmt.Sprintf("status    %s  progress=%d%%", valueOr(job.Status, "-"), job.ProgressPercent),
		fmt.Sprintf("message   %s", valueOr(job.Message, "-")),
		fmt.Sprintf("retry     retryable=%t attempt=%d/%d", job.Retryable, job.Attempt, job.MaxAttempts),
		fmt.Sprintf("state     resumable=%t cancel_requested=%t", job.Resumable, job.CancelRequested),
		fmt.Sprintf("updated   %s", compactTime(job.UpdatedAt)),
	}
	if job.ParentID != "" {
		lines = append(lines, "parent    "+job.ParentID)
	}
	c.printPanel("Model Job", lines, statusColor(job.Status))
	return nil
}

func (c *runtimeChat) printTask(id string) error {
	var payload runtimeTaskPayload
	if err := getJSONValue(c.cfg.addr+"/api/tasks/"+url.PathEscape(id), &payload); err != nil {
		return err
	}
	task := payload.Task
	lines := []string{
		fmt.Sprintf("id        %s", valueOr(task.ID, "-")),
		fmt.Sprintf("type      %s", valueOr(task.Type, "-")),
		fmt.Sprintf("status    %s  progress=%d%%", valueOr(task.Status, "-"), task.ProgressPercent),
		fmt.Sprintf("message   %s", valueOr(task.Message, "-")),
		fmt.Sprintf("state     resumable=%t", task.Resumable),
		fmt.Sprintf("retry     retryable=%t attempt=%d/%d", task.Retryable, task.Attempt, task.MaxAttempts),
		fmt.Sprintf("updated   %s", compactTime(task.UpdatedAt)),
	}
	if task.ParentID != "" {
		lines = append(lines, "parent    "+task.ParentID)
	}
	if task.Error != "" {
		lines = append(lines, "error     "+firstLine(task.Error, c.contentWidth()-12))
	}
	c.printPanel("Lifecycle Task", lines, statusColor(task.Status))
	return nil
}

func (c *runtimeChat) printJobLogs(id string) error {
	var payload runtimeModelJobLogsPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(id)+"/logs?limit=30", &payload); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("job       %s", valueOr(payload.JobID, id)),
		fmt.Sprintf("status    %s  progress=%d%%", valueOr(payload.Status, "-"), payload.ProgressPercent),
		fmt.Sprintf("retry     retryable=%t attempt=%d/%d", payload.Retryable, payload.Attempt, payload.MaxAttempts),
	}
	if payload.WorkerHeartbeat != nil {
		lines = append(lines, fmt.Sprintf("heartbeat %s  %s  %s", compactTime(payload.WorkerHeartbeat.At), valueOr(payload.WorkerHeartbeat.Status, "-"), valueOr(payload.WorkerHeartbeat.Message, "-")))
	}
	if len(payload.Artifacts) > 0 {
		lines = append(lines, fmt.Sprintf("artifacts %d  %s", len(payload.Artifacts), valueOr(payload.Artifacts[0].URI, "-")))
	}
	if payload.Metadata != nil {
		if manifest := strings.TrimSpace(fmt.Sprint(payload.Metadata["artifact_manifest"])); manifest != "" && manifest != "<nil>" {
			lines = append(lines, "manifest  "+firstLine(manifest, c.contentWidth()-12))
		}
	}
	if strings.TrimSpace(payload.Stdout) != "" {
		lines = append(lines, "stdout    "+firstLine(payload.Stdout, c.contentWidth()-12))
	}
	if strings.TrimSpace(payload.Stderr) != "" {
		lines = append(lines, "stderr    "+firstLine(payload.Stderr, c.contentWidth()-12))
	}
	if len(payload.Logs) == 0 {
		lines = append(lines, "", "no logs")
	} else {
		lines = append(lines, "")
		for _, log := range payload.Logs {
			lines = append(lines, modelJobLogLine(log))
		}
	}
	c.printPanel("Model Job Logs", lines, statusColor(payload.Status))
	return nil
}

func (c *runtimeChat) printTaskLogs(id string) error {
	var payload runtimeTaskLogsPayload
	if err := getJSONValue(c.cfg.addr+"/api/tasks/"+url.PathEscape(id)+"/logs?limit=30", &payload); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("task      %s", valueOr(payload.TaskID, id)),
		fmt.Sprintf("type      %s", valueOr(payload.Type, "-")),
		fmt.Sprintf("status    %s  progress=%d%%", valueOr(payload.Status, "-"), payload.ProgressPercent),
		fmt.Sprintf("retry     retryable=%t attempt=%d/%d", payload.Retryable, payload.Attempt, payload.MaxAttempts),
	}
	if payload.WorkerHeartbeat != nil {
		lines = append(lines, fmt.Sprintf("heartbeat %s  %s  %s", compactTime(payload.WorkerHeartbeat.At), valueOr(payload.WorkerHeartbeat.Status, "-"), valueOr(payload.WorkerHeartbeat.Message, "-")))
	}
	if len(payload.Artifacts) > 0 {
		lines = append(lines, fmt.Sprintf("artifacts %d  %s", len(payload.Artifacts), valueOr(payload.Artifacts[0].URI, "-")))
	}
	if payload.Metadata != nil {
		if manifest := strings.TrimSpace(fmt.Sprint(payload.Metadata["artifact_manifest"])); manifest != "" && manifest != "<nil>" {
			lines = append(lines, "manifest  "+firstLine(manifest, c.contentWidth()-12))
		}
	}
	if strings.TrimSpace(payload.Stdout) != "" {
		lines = append(lines, "stdout    "+firstLine(payload.Stdout, c.contentWidth()-12))
	}
	if strings.TrimSpace(payload.Stderr) != "" {
		lines = append(lines, "stderr    "+firstLine(payload.Stderr, c.contentWidth()-12))
	}
	if len(payload.Logs) == 0 {
		lines = append(lines, "", "no logs")
	} else {
		lines = append(lines, "")
		for _, log := range payload.Logs {
			lines = append(lines, taskLogLine(log))
		}
	}
	c.printPanel("Lifecycle Task Logs", lines, statusColor(payload.Status))
	return nil
}

func (c *runtimeChat) printJobManifest(id string) error {
	var payload runtimeArtifactManifestPayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(id)+"/manifest", &payload); err != nil {
		return err
	}
	c.printArtifactManifestPanel("Model Job Artifact Manifest", payload)
	return nil
}

func (c *runtimeChat) printJobLineage(id string) error {
	var payload runtimeJobLineagePayload
	if err := getJSONValue(c.cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(id)+"/lineage", &payload); err != nil {
		return err
	}
	lines := []string{
		"job       " + valueOr(payload.JobID, id),
		"root      " + valueOr(payload.RootID, "-"),
		fmt.Sprintf("count     %d", payload.Count),
		"",
	}
	for _, job := range payload.Lineage {
		line := fmt.Sprintf("%s  %-12s %3d%% %s", valueOr(job.ID, "-"), valueOr(job.Status, "-"), job.ProgressPercent, valueOr(job.Kind, "-"))
		if job.ParentID != "" {
			line += "  parent=" + job.ParentID
		}
		lines = append(lines, line)
	}
	c.printPanel("Model Job Lineage", lines, "yellow")
	return nil
}

func (c *runtimeChat) printTaskManifest(id string) error {
	var payload runtimeArtifactManifestPayload
	if err := getJSONValue(c.cfg.addr+"/api/tasks/"+url.PathEscape(id)+"/manifest", &payload); err != nil {
		return err
	}
	c.printArtifactManifestPanel("Lifecycle Task Artifact Manifest", payload)
	return nil
}

func (c *runtimeChat) printTaskLineage(id string) error {
	var payload runtimeTaskLineagePayload
	if err := getJSONValue(c.cfg.addr+"/api/tasks/"+url.PathEscape(id)+"/lineage", &payload); err != nil {
		return err
	}
	lines := []string{
		"task      " + valueOr(payload.TaskID, id),
		"root      " + valueOr(payload.RootID, "-"),
		fmt.Sprintf("count     %d", payload.Count),
		"",
	}
	for _, task := range payload.Lineage {
		line := fmt.Sprintf("%s  %-12s %3d%% %s", valueOr(task.ID, "-"), valueOr(task.Status, "-"), task.ProgressPercent, valueOr(task.Type, "-"))
		if task.ParentID != "" {
			line += "  parent=" + task.ParentID
		}
		lines = append(lines, line)
	}
	c.printPanel("Lifecycle Task Lineage", lines, "yellow")
	return nil
}

func (c *runtimeChat) printArtifactManifestPanel(title string, payload runtimeArtifactManifestPayload) {
	lines := []string{
		"schema    " + valueOr(payload.Manifest.SchemaVersion, "-"),
		"path      " + valueOr(payload.Path, "-"),
		fmt.Sprintf("count     %d", payload.Manifest.ArtifactSummary.ArtifactCount),
	}
	if primary := payload.Manifest.ArtifactSummary.PrimaryArtifact; primary != nil {
		lines = append(lines,
			"primary   "+valueOr(primary.Name, "-"),
			"uri       "+valueOr(primary.URI, "-"),
			"role      "+valueOr(primary.Role, "-"),
		)
		if strings.TrimSpace(primary.ExecutionMode) != "" {
			lines = append(lines, "mode      "+primary.ExecutionMode)
		}
	}
	if counts := compactCountMap(payload.Manifest.ArtifactSummary.RoleCounts); counts != "" {
		lines = append(lines, "roles     "+counts)
	}
	if counts := compactCountMap(payload.Manifest.ArtifactSummary.ExecutionModeCounts); counts != "" {
		lines = append(lines, "modes     "+counts)
	}
	if len(payload.Manifest.Artifacts) > 0 {
		lines = append(lines, "")
		for _, artifact := range payload.Manifest.Artifacts {
			lines = append(lines, fmt.Sprintf("%s  %s  %s", valueOr(artifact.Name, "-"), valueOr(artifact.Kind, "-"), valueOr(artifact.URI, "-")))
		}
	}
	c.printPanel(title, lines, "yellow")
}

func (c *runtimeChat) followJob(id string) error {
	req, err := http.NewRequest(http.MethodGet, c.cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(id)+"/logs/stream?timeout_ms=60000", nil)
	if err != nil {
		return err
	}
	applyGatewayAuth(req, c.cfg.token)
	client := &http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(bodyBytes))
	}
	c.printPanel("Following Model Job", []string{
		"job       " + id,
		"endpoint  /api/runtime/model-jobs/" + id + "/logs/stream",
	}, "yellow")
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event runtimeJobStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("parse model job stream event: %w: %s", err, line)
		}
		switch event.Type {
		case "log":
			fmt.Fprintf(c.out, "%s\n", modelJobLogLine(event.Log))
		case "update":
			c.printRuntimeJobStreamUpdate(event)
		case "final":
			fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("final status=%s progress=%d%% %s", valueOr(event.Status, "-"), event.ProgressPercent, valueOr(event.Message, "")), statusColor(event.Status)))
			if event.Attempt > 0 || event.MaxAttempts > 0 {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("retry attempt=%d/%d retryable=%t", event.Attempt, event.MaxAttempts, event.Retryable), "dim"))
			}
			if event.WorkerHeartbeat != nil {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("heartbeat %s %s %s", compactTime(event.WorkerHeartbeat.At), valueOr(event.WorkerHeartbeat.Status, "-"), valueOr(event.WorkerHeartbeat.Message, "-")), "dim"))
			}
			if len(event.Artifacts) > 0 {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("artifact  %s", valueOr(event.Artifacts[0].URI, "-")), "dim"))
			}
			if event.Metadata != nil {
				if manifest := strings.TrimSpace(fmt.Sprint(event.Metadata["artifact_manifest"])); manifest != "" && manifest != "<nil>" {
					fmt.Fprintf(c.out, "%s\n", c.color("manifest  "+firstLine(manifest, c.contentWidth()-12), "dim"))
				}
			}
			if strings.TrimSpace(event.Stdout) != "" {
				fmt.Fprintf(c.out, "%s\n", c.color("stdout    "+firstLine(event.Stdout, c.contentWidth()-12), "dim"))
			}
			if strings.TrimSpace(event.Stderr) != "" {
				fmt.Fprintf(c.out, "%s\n", c.color("stderr    "+firstLine(event.Stderr, c.contentWidth()-12), "dim"))
			}
			return nil
		case "error":
			return errors.New(valueOr(event.Message, "model job stream failed"))
		default:
			fmt.Fprintf(c.out, "%s\n", firstLine(line, c.contentWidth()))
		}
	}
	return scanner.Err()
}

func (c *runtimeChat) followTask(id string) error {
	req, err := http.NewRequest(http.MethodGet, c.cfg.addr+"/api/tasks/"+url.PathEscape(id)+"/logs/stream?timeout_ms=60000", nil)
	if err != nil {
		return err
	}
	applyGatewayAuth(req, c.cfg.token)
	client := &http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(bodyBytes))
	}
	c.printPanel("Following Lifecycle Task", []string{
		"task      " + id,
		"endpoint  /api/tasks/" + id + "/logs/stream",
	}, "yellow")
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event runtimeTaskStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("parse lifecycle task stream event: %w: %s", err, line)
		}
		switch event.Type {
		case "log":
			fmt.Fprintf(c.out, "%s\n", taskLogLine(event.Log))
		case "update":
			c.printRuntimeTaskStreamUpdate(event)
		case "final":
			fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("final status=%s progress=%d%% %s", valueOr(event.Status, "-"), event.ProgressPercent, valueOr(event.Message, "")), statusColor(event.Status)))
			if event.Attempt > 0 || event.MaxAttempts > 0 {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("retry attempt=%d/%d retryable=%t", event.Attempt, event.MaxAttempts, event.Retryable), "dim"))
			}
			if event.WorkerHeartbeat != nil {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("heartbeat %s %s %s", compactTime(event.WorkerHeartbeat.At), valueOr(event.WorkerHeartbeat.Status, "-"), valueOr(event.WorkerHeartbeat.Message, "-")), "dim"))
			}
			if len(event.Artifacts) > 0 {
				fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("artifact  %s", valueOr(event.Artifacts[0].URI, "-")), "dim"))
			}
			if event.Metadata != nil {
				if manifest := strings.TrimSpace(fmt.Sprint(event.Metadata["artifact_manifest"])); manifest != "" && manifest != "<nil>" {
					fmt.Fprintf(c.out, "%s\n", c.color("manifest  "+firstLine(manifest, c.contentWidth()-12), "dim"))
				}
			}
			if strings.TrimSpace(event.Stdout) != "" {
				fmt.Fprintf(c.out, "%s\n", c.color("stdout    "+firstLine(event.Stdout, c.contentWidth()-12), "dim"))
			}
			if strings.TrimSpace(event.Stderr) != "" {
				fmt.Fprintf(c.out, "%s\n", c.color("stderr    "+firstLine(event.Stderr, c.contentWidth()-12), "dim"))
			}
			return nil
		case "error":
			return errors.New(valueOr(event.Message, "lifecycle task stream failed"))
		default:
			fmt.Fprintf(c.out, "%s\n", firstLine(line, c.contentWidth()))
		}
	}
	return scanner.Err()
}

func (c *runtimeChat) printRuntimeJobStreamUpdate(event runtimeJobStreamEvent) {
	fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("update status=%s progress=%d%% %s", valueOr(event.Status, "-"), event.ProgressPercent, valueOr(event.Message, "")), "dim"))
	if event.Attempt > 0 || event.MaxAttempts > 0 {
		fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("retry attempt=%d/%d retryable=%t", event.Attempt, event.MaxAttempts, event.Retryable), "dim"))
	}
	if event.WorkerHeartbeat != nil {
		fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("heartbeat %s %s %s", compactTime(event.WorkerHeartbeat.At), valueOr(event.WorkerHeartbeat.Status, "-"), valueOr(event.WorkerHeartbeat.Message, "-")), "dim"))
	}
	if strings.TrimSpace(event.Stdout) != "" {
		fmt.Fprintf(c.out, "%s\n", c.color("stdout    "+firstLine(event.Stdout, c.contentWidth()-12), "dim"))
	}
	if strings.TrimSpace(event.Stderr) != "" {
		fmt.Fprintf(c.out, "%s\n", c.color("stderr    "+firstLine(event.Stderr, c.contentWidth()-12), "dim"))
	}
}

func (c *runtimeChat) printRuntimeTaskStreamUpdate(event runtimeTaskStreamEvent) {
	fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("update status=%s progress=%d%% %s", valueOr(event.Status, "-"), event.ProgressPercent, valueOr(event.Message, "")), "dim"))
	if event.Attempt > 0 || event.MaxAttempts > 0 {
		fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("retry attempt=%d/%d retryable=%t", event.Attempt, event.MaxAttempts, event.Retryable), "dim"))
	}
	if event.WorkerHeartbeat != nil {
		fmt.Fprintf(c.out, "%s\n", c.color(fmt.Sprintf("heartbeat %s %s %s", compactTime(event.WorkerHeartbeat.At), valueOr(event.WorkerHeartbeat.Status, "-"), valueOr(event.WorkerHeartbeat.Message, "-")), "dim"))
	}
	if strings.TrimSpace(event.Stdout) != "" {
		fmt.Fprintf(c.out, "%s\n", c.color("stdout    "+firstLine(event.Stdout, c.contentWidth()-12), "dim"))
	}
	if strings.TrimSpace(event.Stderr) != "" {
		fmt.Fprintf(c.out, "%s\n", c.color("stderr    "+firstLine(event.Stderr, c.contentWidth()-12), "dim"))
	}
}

func (c *runtimeChat) resumeTask(id string) error {
	var payload runtimeTaskPayload
	if err := postJSONValue(c.cfg.addr+"/api/tasks/"+url.PathEscape(id)+"/resume", map[string]string{}, &payload); err != nil {
		return err
	}
	c.printPanel("Lifecycle Task Resumed", []string{
		"id        " + valueOr(payload.Task.ID, "-"),
		"type      " + valueOr(payload.Task.Type, "-"),
		"status    " + valueOr(payload.Task.Status, "-"),
		"message   " + valueOr(payload.Task.Message, "-"),
	}, statusColor(payload.Task.Status))
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
		lines = append(lines, fmt.Sprintf("planner   mode=%s effective=%s transport=%s mimo=%t fallback=%s token=%t", valueOr(status.Runtime.Planner.Mode, "-"), valueOr(status.Runtime.Planner.EffectiveMode, "-"), valueOr(status.Runtime.Planner.Transport, "-"), status.Runtime.Planner.MimoEnabled, valueOr(status.Runtime.Planner.MimoFallback, "-"), status.Runtime.Planner.TokenPresent))
		lines = append(lines, fmt.Sprintf("auth      token=%t remote_requires_token=%t loopback_bypass=%t", status.Gateway.Auth.TokenConfigured, status.Gateway.Auth.RemoteRequiresToken, status.Gateway.Auth.LoopbackBypass))
		lines = append(lines, fmt.Sprintf("python    %s", valueOr(status.Runtime.Planner.PythonPath, "-")))
	}
	lines = append(lines,
		"CLI env",
		"  LLM_BASE_URL          "+presentOrMissing("LLM_BASE_URL"),
		"  LLM_MODEL             "+presentOrMissing("LLM_MODEL"),
		"  LLM_API_KEY           "+presentOrMissing("LLM_API_KEY"),
		"  ANTHROPIC_BASE_URL    "+presentOrMissing("ANTHROPIC_BASE_URL"),
		"  ANTHROPIC_AUTH_TOKEN  "+presentOrMissing("ANTHROPIC_AUTH_TOKEN"),
		"  ATM_GATEWAY_TOKEN     "+presentOrMissing("ATM_GATEWAY_TOKEN"),
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
	case "tasks":
		return getJSON(c.cfg.addr + "/api/tasks")
	default:
		return fmt.Errorf("unknown json target: %s", name)
	}
}

func taskLogLine(log runtimeTaskLog) string {
	return fmt.Sprintf("%s  %-5s  %s", compactTime(log.At), valueOr(log.Level, "info"), valueOr(log.Message, "-"))
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyGatewayAuth(req, cliGatewayToken)
	resp, err := http.DefaultClient.Do(req)
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

func postJSONValue(url string, body any, target any) error {
	rawBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyGatewayAuth(req, cliGatewayToken)
	resp, err := http.DefaultClient.Do(req)
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyGatewayAuth(req, cliGatewayToken)
	resp, err := client.Do(req)
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

func compactCountMap(values map[string]int) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func statusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "succeeded", "ok", "planned", "tool_planned", "tool_planned_with_guard":
		return "green"
	case "running":
		return "cyan"
	case "canceled", "approval_required":
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
