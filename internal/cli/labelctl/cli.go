package labelctl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/skillapp"
)

const (
	defaultAddr = "http://127.0.0.1:7870"
)

type Config struct {
	addr  string
	token string
}

var cliGatewayToken string

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type runtimeSendResponse struct {
	Reply struct {
		Text string `json:"text"`
	} `json:"reply"`
}

type plannedAction struct {
	Action     string            `json:"action"`
	Message    string            `json:"message,omitempty"`
	RepoID     string            `json:"repo_id,omitempty"`
	PullLFS    bool              `json:"pull_lfs,omitempty"`
	WorkflowID string            `json:"workflow_id,omitempty"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	Scene      string            `json:"scene,omitempty"`
	DryRun     bool              `json:"dry_run,omitempty"`
	Endpoint   string            `json:"endpoint,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

func Run(args []string) error {
	cfg := Config{token: firstEnv("ATM_GATEWAY_TOKEN", "GATEWAY_AUTH_TOKEN")}
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "labelserver base URL")
	flag.StringVar(&cfg.token, "token", cfg.token, "Gateway bearer token; defaults to ATM_GATEWAY_TOKEN or GATEWAY_AUTH_TOKEN")
	flag.Parse()
	cliGatewayToken = cfg.token
	if args == nil {
		args = flag.Args()
	}
	if len(args) == 0 {
		usage()
		return errors.New("missing command")
	}
	return dispatch(cfg, args)
}

func dispatch(cfg Config, args []string) error {
	switch args[0] {
	case "help", "-h", "--help":
		usage()
	case "health":
		return getJSON(cfg.addr + "/healthz")
	case "videos":
		return getJSON(cfg.addr + "/api/videos")
	case "providers":
		return getJSON(cfg.addr + "/api/providers")
	case "secrets":
		return getJSON(cfg.addr + "/api/secrets")
	case "agents":
		return getJSON(cfg.addr + "/api/agents")
	case "workflows":
		return getJSON(cfg.addr + "/api/workflows")
	case "runs":
		return getJSON(cfg.addr + "/api/agent-runs")
	case "audit":
		return getJSON(cfg.addr + "/api/audit-events")
	case "dataset", "datasets":
		return runDataset(cfg, args[1:])
	case "autolabel":
		return runAutoLabel(cfg, args[1:])
	case "training", "train":
		return runTraining(cfg, args[1:])
	case "evaluation", "eval":
		return runEvaluation(cfg, args[1:])
	case "models", "model":
		return runModels(cfg, args[1:])
	case "deploy", "deployment":
		return runDeploy(cfg, args[1:])
	case "logs", "log":
		return runLogs(cfg, args[1:])
	case "doctor":
		return runDoctor(cfg, args[1:])
	case "runtime":
		return runRuntime(cfg, args[1:])
	case "desktop":
		return runDesktop(cfg, args[1:])
	case "channels":
		return getJSON(cfg.addr + "/api/channels")
	case "channel":
		return runChannel(cfg, args[1:])
	case "governance":
		return runGovernance(cfg, args[1:])
	case "agent":
		return runAgentAssistant(cfg, args[1:])
	case "ask":
		return runAsk(strings.Join(args[1:], " "))
	case "video":
		if len(args) < 2 {
			return errors.New("usage: labelctl video <scene>")
		}
		return getJSON(cfg.addr + "/api/video/" + args[1] + "/meta")
	case "agent-run":
		return runAgentWorkflow(cfg, args[1:])
	case "skill":
		return runSkill(args[1:])
	case "llm":
		return runLLM(cfg, args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
	return nil
}

func runAgentAssistant(cfg Config, args []string) error {
	if len(args) == 0 {
		return runRuntimeChat(cfg)
	}
	switch args[0] {
	case "ask":
		return sendRuntimeText(cfg, strings.Join(args[1:], " "), true)
	case "run":
		return runAgentWorkflow(cfg, args[1:])
	case "auto":
		return runLLMAgent(cfg, true)
	case "llm":
		return runLLMAgent(cfg, false)
	default:
		return sendRuntimeText(cfg, strings.Join(args, " "), true)
	}
}

func runAsk(prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return errors.New("usage: labelctl ask <prompt> | labelctl agent ask <prompt>")
	}
	answer, err := callLLM(context.Background(), []chatMessage{{Role: "user", Content: prompt}}, 0.2)
	if err != nil {
		return err
	}
	fmt.Println(answer)
	return nil
}

func runGovernance(cfg Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: labelctl governance all|enforcement|data|release|runtime")
	}
	switch args[0] {
	case "all":
		return getJSON(cfg.addr + "/api/governance/control-surface")
	case "enforcement":
		return getJSON(cfg.addr + "/api/governance/enforcement-points")
	case "data":
		return getJSON(cfg.addr + "/api/governance/data-policies")
	case "release":
		return getJSON(cfg.addr + "/api/governance/release-policies")
	case "runtime":
		return getJSON(cfg.addr + "/api/governance/runtime-policies")
	default:
		return fmt.Errorf("unknown governance command: %s", args[0])
	}
}

func runRuntime(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "status" {
		return getJSON(cfg.addr + "/api/runtime/status")
	}
	if args[0] == "sessions" {
		return getJSON(cfg.addr + "/api/runtime/sessions")
	}
	if args[0] == "traces" {
		return getJSON(cfg.addr + "/api/runtime/traces")
	}
	if args[0] == "model-jobs" || args[0] == "jobs" {
		return getJSON(cfg.addr + "/api/runtime/model-jobs")
	}
	if args[0] == "intake-workflows" || args[0] == "intake" {
		return getJSON(cfg.addr + "/api/runtime/intake/workflows")
	}
	if args[0] == "intake-workflow" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime intake-workflow <workflow_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/intake/workflows/" + url.PathEscape(args[1]))
	}
	if args[0] == "approve-intake" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime approve-intake <workflow_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/intake/workflows/"+url.PathEscape(args[1])+"/approve", map[string]string{"by": "labelctl"})
	}
	if args[0] == "register-intake" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime register-intake <workflow_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/intake/workflows/"+url.PathEscape(args[1])+"/register", map[string]string{"by": "labelctl"})
	}
	if args[0] == "job" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime job <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]))
	}
	if args[0] == "job-logs" || args[0] == "logs-job" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime job-logs <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]) + "/logs")
	}
	if args[0] == "job-logs-stream" || args[0] == "follow-job" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime job-logs-stream <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]) + "/logs/stream")
	}
	if args[0] == "cancel-job" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime cancel-job <job_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(args[1])+"/cancel", map[string]string{})
	}
	if args[0] == "resume-job" {
		if len(args) < 2 {
			return errors.New("usage: labelctl runtime resume-job <job_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(args[1])+"/resume", map[string]string{})
	}
	if args[0] == "send" {
		text := strings.TrimSpace(strings.Join(args[1:], " "))
		if text == "" {
			return errors.New("usage: labelctl runtime send <message>")
		}
		return sendRuntimeMessage(cfg, text)
	}
	if args[0] == "chat" || args[0] == "agent" {
		if len(args) > 1 {
			return sendRuntimeText(cfg, strings.Join(args[1:], " "), true)
		}
		return runRuntimeChat(cfg)
	}
	return fmt.Errorf("unknown runtime command: %s", args[0])
}

func sendRuntimeMessage(cfg Config, text string) error {
	body := map[string]any{
		"id":         fmt.Sprintf("cli_runtime_%d", time.Now().UnixNano()),
		"channel":    "qq",
		"account_id": "default",
		"peer": map[string]any{
			"channel":    "qq",
			"account_id": "default",
			"kind":       "direct",
			"id":         "cli-runtime",
		},
		"sender_id": "cli-runtime",
		"text":      text,
	}
	return postJSON(cfg.addr+"/api/channels/qq/test-message", body)
}

func sendRuntimeText(cfg Config, text string, plain bool) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty runtime message")
	}
	resp, err := postRuntimeMessage(cfg, text)
	if err != nil {
		return err
	}
	if plain {
		fmt.Println(resp.Reply.Text)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func postRuntimeMessage(cfg Config, text string) (runtimeSendResponse, error) {
	raw, err := runtimeInboundMessageBody(text, "cli-runtime")
	if err != nil {
		return runtimeSendResponse{}, err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.addr+"/api/channels/qq/test-message", bytes.NewReader(raw))
	if err != nil {
		return runtimeSendResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyGatewayAuth(req, cfg.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return runtimeSendResponse{}, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return runtimeSendResponse{}, err
	}
	if resp.StatusCode >= 400 {
		return runtimeSendResponse{}, fmt.Errorf("%s: %s", resp.Status, string(bodyBytes))
	}
	var parsed runtimeSendResponse
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return runtimeSendResponse{}, fmt.Errorf("parse runtime reply: %w\n%s", err, string(bodyBytes))
	}
	return parsed, nil
}

func runtimeInboundMessageBody(text string, peerID string) ([]byte, error) {
	if strings.TrimSpace(peerID) == "" {
		peerID = "cli-runtime"
	}
	body := map[string]any{
		"id":         fmt.Sprintf("cli_runtime_%d", time.Now().UnixNano()),
		"channel":    "qq",
		"account_id": "default",
		"peer": map[string]any{
			"channel":    "qq",
			"account_id": "default",
			"kind":       "direct",
			"id":         peerID,
		},
		"sender_id": "cli-runtime",
		"text":      text,
	}
	return json.Marshal(body)
}

func runRuntimeChat(cfg Config) error {
	return newRuntimeChat(cfg, os.Stdin, os.Stdout, os.Stderr).Run()
}

func runDesktop(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "status" {
		return getJSON(cfg.addr + "/api/desktop/status")
	}
	switch args[0] {
	case "json":
		return getJSON(cfg.addr + "/api/desktop/status")
	case "sessions":
		return getJSON(cfg.addr + "/api/runtime/sessions")
	case "traces":
		return getJSON(cfg.addr + "/api/runtime/traces")
	case "jobs", "model-jobs":
		return getJSON(cfg.addr + "/api/runtime/model-jobs")
	case "send":
		text := strings.TrimSpace(strings.Join(args[1:], " "))
		if text == "" {
			return errors.New("usage: labelctl desktop send <message>")
		}
		return sendRuntimeMessage(cfg, text)
	}
	return fmt.Errorf("unknown desktop command: %s", args[0])
}

func runChannel(cfg Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: labelctl channel qq status|test [message]")
	}
	switch args[0] {
	case "qq":
		return runQQChannel(cfg, args[1:])
	default:
		return fmt.Errorf("unknown channel: %s", args[0])
	}
}

func runQQChannel(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "status" {
		return getJSON(cfg.addr + "/api/channels/qq/status")
	}
	switch args[0] {
	case "test":
		text := strings.TrimSpace(strings.Join(args[1:], " "))
		if text == "" {
			text = "/bot-ping"
		}
		body := map[string]any{
			"id":         fmt.Sprintf("cli_%d", time.Now().UnixNano()),
			"channel":    "qq",
			"account_id": "default",
			"peer": map[string]any{
				"channel":    "qq",
				"account_id": "default",
				"kind":       "direct",
				"id":         "cli-test",
			},
			"sender_id": "cli-test",
			"text":      text,
		}
		return postJSON(cfg.addr+"/api/channels/qq/test-message", body)
	default:
		return fmt.Errorf("unknown qq channel command: %s", args[0])
	}
}

func runAgentWorkflow(cfg Config, args []string) error {
	fs := flag.NewFlagSet("agent-run", flag.ExitOnError)
	workflowID := fs.String("workflow", "data-to-deployment-lifecycle", "workflow id")
	datasetID := fs.String("dataset", "", "dataset id")
	scene := fs.String("scene", "", "scene id")
	dryRun := fs.Bool("dry-run", true, "submit as dry-run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	body := map[string]any{
		"workflow_id": *workflowID,
		"dataset_id":  *datasetID,
		"scene":       *scene,
		"dry_run":     *dryRun,
		"params": map[string]string{
			"source": "labelctl",
		},
	}
	return postJSON(cfg.addr+"/api/agent-runs", body)
}

func runSkill(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: labelctl skill download-model ... | draft|drafts|approve-draft|reject-draft ...")
	}
	switch args[0] {
	case "download-model":
		return runDownloadModelSkill(args[1:])
	case "draft":
		return runDraftSkill(args[1:])
	case "drafts", "list-drafts":
		return runListSkillDrafts(args[1:])
	case "approve-draft":
		return runReviewSkillDraft(args[1:], true)
	case "reject-draft":
		return runReviewSkillDraft(args[1:], false)
	default:
		return fmt.Errorf("unknown skill command: %s", args[0])
	}
}

func runDraftSkill(args []string) error {
	fs := flag.NewFlagSet("skill draft", flag.ExitOnError)
	id := fs.String("id", "", "skill id, for example qq-data-intake")
	title := fs.String("title", "", "human-readable skill title")
	summary := fs.String("summary", "", "short summary of the successful workflow")
	draftRoot := fs.String("draft-root", filepath.Join("data_lake", "agents", "skill_drafts"), "draft skill root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	skillID := strings.TrimSpace(*id)
	if skillID == "" {
		return errors.New("-id is required")
	}
	skillTitle := strings.TrimSpace(*title)
	if skillTitle == "" {
		skillTitle = skillID
	}
	skillSummary := strings.TrimSpace(*summary)
	if skillSummary == "" {
		return errors.New("-summary is required")
	}
	draft, err := skillapp.NewService(time.Now).Draft(skillapp.DraftRequest{
		ID:      skillID,
		Title:   skillTitle,
		Summary: skillSummary,
		Root:    *draftRoot,
	})
	if err != nil {
		return err
	}
	fmt.Println("draft skill written:", draft.Path)
	fmt.Println("status:", draft.Status)
	fmt.Println("enabled:", draft.Enabled)
	return nil
}

func runListSkillDrafts(args []string) error {
	fs := flag.NewFlagSet("skill drafts", flag.ExitOnError)
	draftRoot := fs.String("draft-root", filepath.Join("data_lake", "agents", "skill_drafts"), "draft skill root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	drafts, err := skillapp.NewService(time.Now).List(*draftRoot)
	if err != nil {
		return err
	}
	if len(drafts) == 0 {
		fmt.Println("no skill drafts")
		return nil
	}
	for _, draft := range drafts {
		fmt.Printf("%s\t%s\tenabled=%t\t%s\n", draft.ID, draft.Status, draft.Enabled, draft.Path)
	}
	return nil
}

func runReviewSkillDraft(args []string, approve bool) error {
	name := "skill reject-draft"
	if approve {
		name = "skill approve-draft"
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	draftRoot := fs.String("draft-root", filepath.Join("data_lake", "agents", "skill_drafts"), "draft skill root")
	by := fs.String("by", "labelctl", "reviewer id")
	note := fs.String("note", "", "review note")
	skillID := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		skillID = args[0]
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if skillID == "" {
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: labelctl %s <skill-id> [-by reviewer] [-note note]", name)
		}
		skillID = fs.Arg(0)
	}
	req := skillapp.ReviewRequest{ID: skillID, Root: *draftRoot, By: *by, Note: *note}
	var (
		record skillapp.ReviewRecord
		err    error
	)
	if approve {
		record, err = skillapp.NewService(time.Now).Approve(req)
	} else {
		record, err = skillapp.NewService(time.Now).Reject(req)
	}
	if err != nil {
		return err
	}
	fmt.Printf("skill draft %s: %s by %s\n", record.SkillID, record.Status, record.By)
	fmt.Println("enabled:", record.Enabled)
	return nil
}

func runDownloadModelSkill(args []string) error {
	fs := flag.NewFlagSet("skill download-model", flag.ExitOnError)
	repoID := fs.String("repo", "", "Hugging Face repo id in org/repo form")
	pullLFS := fs.Bool("pull-lfs", false, "download Git LFS model weights")
	dryRun := fs.Bool("dry-run", false, "print the script command without executing it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*repoID) == "" {
		return errors.New("-repo is required")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(*repoID) {
		return fmt.Errorf("invalid repo id: %s", *repoID)
	}
	script := filepath.Join("skills", "automated-training-data-lake", "scripts", "download_hf_model.ps1")
	cmdArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-RepoId", *repoID}
	if *pullLFS {
		cmdArgs = append(cmdArgs, "-PullLFS")
	}
	fmt.Println("skill: automated-training-data-lake")
	fmt.Println("destination:", filepath.Join("data_lake", "models", "artifacts", "huggingface", strings.ReplaceAll(*repoID, "/", string(os.PathSeparator))))
	if *dryRun {
		fmt.Println("command: powershell", strings.Join(cmdArgs, " "))
		return nil
	}
	cmd := exec.Command("powershell", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runLLM(cfg Config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: labelctl llm ask <prompt> | labelctl llm agent [-auto]")
	}
	switch args[0] {
	case "ask":
		return runAsk(strings.Join(args[1:], " "))
	case "agent":
		fs := flag.NewFlagSet("llm agent", flag.ExitOnError)
		auto := fs.Bool("auto", false, "execute planned actions without confirmation")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runLLMAgent(cfg, *auto)
	default:
		return fmt.Errorf("unknown llm command: %s", args[0])
	}
}

func runLLMAgent(cfg Config, auto bool) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("LLM agent ready. Type a command, or 'exit'.")
	for {
		fmt.Print("atm> ")
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return nil
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		input := strings.TrimSpace(line)
		if input == "" {
			if errors.Is(err, io.EOF) {
				return nil
			}
			continue
		}
		if input == "exit" || input == "quit" {
			return nil
		}
		action, err := planAction(input)
		if err != nil {
			return err
		}
		if err := printPlannedAction(action); err != nil {
			return err
		}
		if !auto {
			fmt.Print("execute? [y/N] ")
			confirm, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("skipped")
				continue
			}
		}
		if err := executeAction(cfg, action); err != nil {
			fmt.Fprintln(os.Stderr, "action failed:", err)
		}
	}
}

func runLLMAgentWithPrompt(cfg Config, auto bool, input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return runLLMAgent(cfg, auto)
	}
	action, err := planAction(input)
	if err != nil {
		return err
	}
	if err := printPlannedAction(action); err != nil {
		return err
	}
	if !auto {
		fmt.Print("execute? [y/N] ")
		confirmReader := bufio.NewReader(os.Stdin)
		confirm, _ := confirmReader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			fmt.Println("skipped")
			return nil
		}
	}
	return executeAction(cfg, action)
}

func planAction(input string) (plannedAction, error) {
	system := `You are a strict CLI action planner for the Automated Training Model project.
Return JSON only. No markdown.
Allowed schemas:
{"action":"chat","message":"short answer"}
{"action":"download_hf_model","repo_id":"org/repo","pull_lfs":true}
{"action":"agent_run","workflow_id":"data-to-deployment-lifecycle","dataset_id":"...","scene":"...","dry_run":true,"params":{"source":"cli-agent"}}
{"action":"api_get","endpoint":"/api/agents"}
Only choose api_get for safe read-only endpoints under /healthz, /api/runtime/status, /api/runtime/sessions, /api/runtime/traces, /api/runtime/model-jobs, /api/desktop/status, /api/channels, /api/channels/qq/status, /api/agents, /api/tools, /api/workflows, /api/agent-runs, /api/audit-events, /api/videos, /api/datasets, /api/models, /api/governance/control-surface, /api/governance/enforcement-points, /api/governance/data-policies, /api/governance/release-policies, /api/governance/runtime-policies. Use runtime_send for sending a message through the Agent Runtime test channel.
The CLI agent is the primary interface. Prefer workflow_id "data-to-deployment-lifecycle" for full lifecycle work from data collection to model deployment. Use "human-loop-autolabel" only when the user explicitly asks for video labeling or review.
For download_hf_model, only use a repository id explicitly present in the user input. If it is missing, return chat asking for the repository id.`
	content, err := callLLM(context.Background(), []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: input},
	}, 0)
	if err != nil {
		return plannedAction{}, err
	}
	var action plannedAction
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &action); err != nil {
		return plannedAction{}, fmt.Errorf("parse LLM action: %w\nraw response: %s", err, content)
	}
	if action.Action == "" {
		return plannedAction{}, errors.New("LLM returned empty action")
	}
	return action, nil
}

func executeAction(cfg Config, action plannedAction) error {
	switch action.Action {
	case "chat":
		fmt.Println(action.Message)
	case "download_hf_model":
		args := []string{"-repo", action.RepoID}
		if action.PullLFS {
			args = append(args, "-pull-lfs")
		}
		return runDownloadModelSkill(args)
	case "agent_run":
		workflowID := strings.TrimSpace(action.WorkflowID)
		if workflowID == "" {
			workflowID = "data-to-deployment-lifecycle"
		}
		params := action.Params
		if params == nil {
			params = map[string]string{}
		}
		if params["source"] == "" {
			params["source"] = "cli-agent"
		}
		body := map[string]any{
			"workflow_id": workflowID,
			"dataset_id":  action.DatasetID,
			"scene":       action.Scene,
			"dry_run":     action.DryRun,
			"params":      params,
		}
		return postJSON(cfg.addr+"/api/agent-runs", body)
	case "api_get":
		if !safeEndpoint(action.Endpoint) {
			return fmt.Errorf("unsafe endpoint: %s", action.Endpoint)
		}
		return getJSON(cfg.addr + action.Endpoint)
	default:
		return fmt.Errorf("unsupported action: %s", action.Action)
	}
	return nil
}

func printPlannedAction(action plannedAction) error {
	raw, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func callLLM(ctx context.Context, messages []chatMessage, temperature float64) (string, error) {
	baseURL := strings.TrimRight(firstEnv("LLM_BASE_URL"), "/")
	if baseURL == "" {
		return "", errors.New("missing LLM_BASE_URL")
	}
	model := firstEnv("LLM_MODEL")
	if model == "" {
		return "", errors.New("missing LLM_MODEL")
	}
	apiKey := firstEnv("LLM_API_KEY")
	reqBody := chatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse LLM response: %w\n%s", err, string(body))
	}
	if resp.StatusCode >= 400 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("LLM API error %s: %s", resp.Status, parsed.Error.Message)
		}
		return "", fmt.Errorf("LLM API error %s: %s", resp.Status, string(body))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("LLM response has no choices: %s", string(body))
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func getJSON(url string) error {
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
	return writeResponse(resp)
}

func postJSON(url string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
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
	return writeResponse(resp)
}

func applyGatewayAuth(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Gateway-Token", token)
}

func writeResponse(resp *http.Response) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: %s", resp.Status, string(raw))
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func safeEndpoint(endpoint string) bool {
	switch endpoint {
	case "/healthz", "/api/runtime/status", "/api/runtime/sessions", "/api/runtime/traces", "/api/runtime/model-jobs", "/api/desktop/status", "/api/channels", "/api/channels/qq/status", "/api/agents", "/api/tools", "/api/workflows", "/api/agent-runs", "/api/audit-events", "/api/videos", "/api/datasets", "/api/models", "/api/governance/control-surface", "/api/governance/enforcement-points", "/api/governance/data-policies", "/api/governance/release-policies", "/api/governance/runtime-policies":
		return true
	default:
		return false
	}
}

func extractJSONObject(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return value
	}
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start >= 0 && end > start {
		return value[start : end+1]
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

func firstEnvWithDefault(fallback string, names ...string) string {
	if value := firstEnv(names...); value != "" {
		return value
	}
	return fallback
}

func usage() {
	fmt.Println(`labelctl commands:
  agent                         start Claude Code style Agent Runtime CLI
  agent <message>               send one message to Agent Runtime
  health
  videos
  dataset [list|register-folder|register-manifest|activate]
  models [list|get <id>|register|jobs|job <id>|job-logs <id>|cancel-job <id>|resume-job <id>]
  deploy [submit|task <id>|task-logs <id>|cancel-task <id>]
  logs [traces|audit|runs|jobs|job <id>|task <id>|intake]
  doctor
  providers
  secrets
  agents
  workflows
  runs
  audit
  runtime [status|sessions|traces|model-jobs|job <id>|job-logs <id>|cancel-job <id>|resume-job <id>|intake|intake-workflow <id>|approve-intake <id>|register-intake <id>|send <message>|chat]
  desktop [status|sessions|traces|jobs|send <message>|json]
  channels
  channel qq status
  channel qq test [/bot-ping]
  governance all|enforcement|data|release|runtime
  agent [ask <prompt> | run [-workflow data-to-deployment-lifecycle] | auto | llm | <prompt>]
  ask <prompt>
  video <scene>

  agent-run -workflow data-to-deployment-lifecycle -dataset <dataset-id> -scene <scene> [-dry-run=true]
  skill download-model -repo <org/repo> [-pull-lfs] [-dry-run]
  skill draft -id <skill-id> -summary <workflow-summary> [-title <title>]
  skill drafts [-draft-root <path>]
  skill approve-draft <skill-id> [-by reviewer] [-note note]
  skill reject-draft <skill-id> [-by reviewer] [-note note]
  llm ask <prompt>
  llm agent [-auto]

LLM env:
  LLM_BASE_URL  Chat-completions compatible base URL
  LLM_MODEL     Chat model id for planning and answers
  LLM_API_KEY   Optional bearer token when the endpoint requires auth

options:
  -addr http://127.0.0.1:7870
  -token <gateway-token>   or set ATM_GATEWAY_TOKEN`)
}
