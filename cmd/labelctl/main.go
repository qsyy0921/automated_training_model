package main

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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultAddr = "http://127.0.0.1:7870"
)

type appConfig struct {
	addr string
}

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

func main() {
	cfg := appConfig{}
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "labelserver base URL")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if err := dispatch(cfg, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(cfg appConfig, args []string) error {
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

func runAgentAssistant(cfg appConfig, args []string) error {
	if len(args) == 0 {
		return runLLMAgent(cfg, false)
	}
	switch args[0] {
	case "ask":
		return runAsk(strings.Join(args[1:], " "))
	case "run":
		return runAgentWorkflow(cfg, args[1:])
	case "auto":
		return runLLMAgent(cfg, true)
	default:
		return runLLMAgentWithPrompt(cfg, false, strings.Join(args, " "))
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

func runGovernance(cfg appConfig, args []string) error {
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

func runAgentWorkflow(cfg appConfig, args []string) error {
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
		return errors.New("usage: labelctl skill download-model -repo <org/repo> [-pull-lfs] [-dry-run]")
	}
	switch args[0] {
	case "download-model":
		return runDownloadModelSkill(args[1:])
	default:
		return fmt.Errorf("unknown skill command: %s", args[0])
	}
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

func runLLM(cfg appConfig, args []string) error {
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

func runLLMAgent(cfg appConfig, auto bool) error {
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

func runLLMAgentWithPrompt(cfg appConfig, auto bool, input string) error {
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
Only choose api_get for safe read-only endpoints under /healthz, /api/agents, /api/tools, /api/workflows, /api/agent-runs, /api/audit-events, /api/videos, /api/governance/control-surface, /api/governance/enforcement-points, /api/governance/data-policies, /api/governance/release-policies, /api/governance/runtime-policies.
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

func executeAction(cfg appConfig, action plannedAction) error {
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
	resp, err := http.Get(url)
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
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return writeResponse(resp)
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
	case "/healthz", "/api/agents", "/api/tools", "/api/workflows", "/api/agent-runs", "/api/audit-events", "/api/videos", "/api/governance/control-surface", "/api/governance/enforcement-points", "/api/governance/data-policies", "/api/governance/release-policies", "/api/governance/runtime-policies":
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
  health
  videos
  providers
  secrets
  agents
  workflows
  runs
  audit
  governance all|enforcement|data|release|runtime
  agent [ask <prompt> | run [-workflow data-to-deployment-lifecycle] | auto | <prompt>]
  ask <prompt>
  video <scene>

  agent-run -workflow data-to-deployment-lifecycle -dataset <dataset-id> -scene <scene> [-dry-run=true]
  skill download-model -repo <org/repo> [-pull-lfs] [-dry-run]
  llm ask <prompt>
  llm agent [-auto]

LLM env:
  LLM_BASE_URL  Chat-completions compatible base URL
  LLM_MODEL     Chat model id for planning and answers
  LLM_API_KEY   Optional bearer token when the endpoint requires auth

options:
  -addr http://127.0.0.1:7870`)
}
