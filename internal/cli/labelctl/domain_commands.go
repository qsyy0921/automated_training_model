package labelctl

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func runDataset(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		return getJSON(cfg.addr + "/api/datasets")
	}
	switch args[0] {
	case "register-folder":
		fs := flag.NewFlagSet("dataset register-folder", flag.ExitOnError)
		name := fs.String("name", "", "dataset name")
		mergeRoot := fs.String("merge-root", "", "tracking merge root")
		frameRoot := fs.String("frame-root", "", "frame root")
		maskRoot := fs.String("mask-root", "", "mask root")
		annotationRoot := fs.String("annotation-root", "", "annotation root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*mergeRoot) == "" {
			return errors.New("usage: labelctl dataset register-folder -merge-root <dir> [-name name] [-frame-root dir] [-mask-root dir] [-annotation-root dir]")
		}
		return postJSON(cfg.addr+"/api/datasets/register-folder", compactBody(map[string]any{
			"name":            *name,
			"merge_root":      *mergeRoot,
			"frame_root":      *frameRoot,
			"mask_root":       *maskRoot,
			"annotation_root": *annotationRoot,
		}))
	case "register-manifest":
		fs := flag.NewFlagSet("dataset register-manifest", flag.ExitOnError)
		name := fs.String("name", "", "dataset name")
		manifest := fs.String("manifest", "", "manifest path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*manifest) == "" {
			return errors.New("usage: labelctl dataset register-manifest -manifest <file> [-name name]")
		}
		return postJSON(cfg.addr+"/api/datasets/register-manifest", compactBody(map[string]any{
			"name":          *name,
			"manifest_path": *manifest,
		}))
	case "activate":
		if len(args) < 2 {
			return errors.New("usage: labelctl dataset activate <dataset_id>")
		}
		return postJSON(cfg.addr+"/api/datasets/"+url.PathEscape(args[1])+"/activate", map[string]string{})
	default:
		return fmt.Errorf("unknown dataset command: %s", args[0])
	}
}

func runAutoLabel(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "help" {
		return errors.New("usage: labelctl autolabel submit -dataset <id> -task-types a,b [-video-ids x,y] [-model-profile p] [-require-review] [-dry-run=true] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec] | task <task_id> | task-logs <task_id> | follow-task <task_id> | cancel-task <task_id>")
	}
	switch args[0] {
	case "submit":
		fs := flag.NewFlagSet("autolabel submit", flag.ExitOnError)
		datasetID := fs.String("dataset", "", "dataset id")
		videoIDs := fs.String("video-ids", "", "comma-separated video ids")
		taskTypes := fs.String("task-types", "", "comma-separated task types")
		modelProfile := fs.String("model-profile", "", "worker model profile")
		prompt := fs.String("prompt", "", "operator prompt")
		requireReview := fs.Bool("require-review", false, "require human review")
		dryRun := fs.Bool("dry-run", true, "submit as dry-run recipe only")
		execSpec := registerExecutionFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*datasetID) == "" || strings.TrimSpace(*taskTypes) == "" {
			return errors.New("usage: labelctl autolabel submit -dataset <id> -task-types a,b [-video-ids x,y] [-model-profile p] [-require-review] [-dry-run=true] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec]")
		}
		body := compactBody(map[string]any{
			"dataset_id":     *datasetID,
			"video_ids":      splitCSV(*videoIDs),
			"task_types":     splitCSV(*taskTypes),
			"model_profile":  *modelProfile,
			"prompt":         *prompt,
			"require_review": *requireReview,
			"dry_run":        *dryRun,
		})
		mergeExecutionBody(body, execSpec)
		return postJSON(cfg.addr+"/api/autolabel/jobs", body)
	case "task", "status":
		if len(args) < 2 {
			return errors.New("usage: labelctl autolabel task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	case "task-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl autolabel task-logs <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs")
	case "follow-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl autolabel follow-task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs/stream")
	case "cancel-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl autolabel cancel-task <task_id>")
		}
		return deleteJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	default:
		return fmt.Errorf("unknown autolabel command: %s", args[0])
	}
}

func runTraining(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "help" {
		return errors.New("usage: labelctl training submit -dataset <id> -target-task <task> -model-family <family> [-annotation-version v] [-split-config name] [-output-registry uri] [-dry-run=false] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec] | task <task_id> | task-logs <task_id> | follow-task <task_id> | cancel-task <task_id>")
	}
	switch args[0] {
	case "submit":
		fs := flag.NewFlagSet("training submit", flag.ExitOnError)
		datasetID := fs.String("dataset", "", "dataset id")
		annotationVersion := fs.String("annotation-version", "", "annotation version")
		targetTask := fs.String("target-task", "", "target task")
		modelFamily := fs.String("model-family", "", "model family")
		baseModel := fs.String("base-model", "", "base model")
		splitConfig := fs.String("split-config", "", "split config")
		outputRegistry := fs.String("output-registry", "", "output registry")
		dryRun := fs.Bool("dry-run", false, "submit as dry-run recipe only")
		execSpec := registerExecutionFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*datasetID) == "" || strings.TrimSpace(*targetTask) == "" || strings.TrimSpace(*modelFamily) == "" {
			return errors.New("usage: labelctl training submit -dataset <id> -target-task <task> -model-family <family> [-annotation-version v] [-split-config name] [-output-registry uri] [-dry-run=false] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec]")
		}
		body := compactBody(map[string]any{
			"dataset_id":         *datasetID,
			"annotation_version": *annotationVersion,
			"target_task":        *targetTask,
			"model_family":       *modelFamily,
			"base_model":         *baseModel,
			"split_config":       *splitConfig,
			"output_registry":    *outputRegistry,
			"dry_run":            *dryRun,
		})
		mergeExecutionBody(body, execSpec)
		return postJSON(cfg.addr+"/api/training/runs", body)
	case "task", "status":
		if len(args) < 2 {
			return errors.New("usage: labelctl training task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	case "task-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl training task-logs <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs")
	case "follow-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl training follow-task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs/stream")
	case "cancel-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl training cancel-task <task_id>")
		}
		return deleteJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	default:
		return fmt.Errorf("unknown training command: %s", args[0])
	}
}

func runEvaluation(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "help" {
		return errors.New("usage: labelctl evaluation submit -dataset <id> -model <id> [-split name] [-metrics a,b] [-save-visuals] [-failure-mining] [-dry-run=false] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec] | task <task_id> | task-logs <task_id> | follow-task <task_id> | cancel-task <task_id>")
	}
	switch args[0] {
	case "submit":
		fs := flag.NewFlagSet("evaluation submit", flag.ExitOnError)
		datasetID := fs.String("dataset", "", "dataset id")
		modelID := fs.String("model", "", "model id")
		split := fs.String("split", "", "split")
		metrics := fs.String("metrics", "", "comma-separated metrics")
		saveVisuals := fs.Bool("save-visuals", false, "save visuals")
		failureMining := fs.Bool("failure-mining", false, "enable failure mining")
		dryRun := fs.Bool("dry-run", false, "submit as dry-run recipe only")
		execSpec := registerExecutionFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*datasetID) == "" || strings.TrimSpace(*modelID) == "" {
			return errors.New("usage: labelctl evaluation submit -dataset <id> -model <id> [-split name] [-metrics a,b] [-save-visuals] [-failure-mining] [-dry-run=false] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec]")
		}
		body := compactBody(map[string]any{
			"dataset_id":     *datasetID,
			"model_id":       *modelID,
			"split":          *split,
			"metrics":        splitCSV(*metrics),
			"save_visuals":   *saveVisuals,
			"failure_mining": *failureMining,
			"dry_run":        *dryRun,
		})
		mergeExecutionBody(body, execSpec)
		return postJSON(cfg.addr+"/api/evaluation/runs", body)
	case "task", "status":
		if len(args) < 2 {
			return errors.New("usage: labelctl evaluation task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	case "task-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl evaluation task-logs <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs")
	case "follow-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl evaluation follow-task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs/stream")
	case "cancel-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl evaluation cancel-task <task_id>")
		}
		return deleteJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	default:
		return fmt.Errorf("unknown evaluation command: %s", args[0])
	}
}

func runModels(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		return getJSON(cfg.addr + "/api/models")
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			return errors.New("usage: labelctl models get <model_id>")
		}
		return getJSON(cfg.addr + "/api/models/" + url.PathEscape(args[1]))
	case "register":
		fs := flag.NewFlagSet("models register", flag.ExitOnError)
		name := fs.String("name", "", "model name")
		family := fs.String("family", "", "model family")
		task := fs.String("task", "", "task type")
		artifactURI := fs.String("artifact-uri", "", "artifact URI or local data_lake path")
		metricsURI := fs.String("metrics-uri", "", "metrics URI")
		datasetID := fs.String("dataset-id", "", "dataset id")
		tags := fs.String("tags", "", "comma-separated tags")
		description := fs.String("description", "", "description")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" || strings.TrimSpace(*artifactURI) == "" {
			return errors.New("usage: labelctl models register -name <name> -artifact-uri <uri> [-family family] [-task task] [-dataset-id id] [-tags a,b]")
		}
		return postJSON(cfg.addr+"/api/models/register", compactBody(map[string]any{
			"name":         *name,
			"model_family": *family,
			"task":         *task,
			"artifact_uri": *artifactURI,
			"metrics_uri":  *metricsURI,
			"dataset_id":   *datasetID,
			"tags":         splitCSV(*tags),
			"description":  *description,
		}))
	case "jobs", "model-jobs":
		return getJSON(cfg.addr + "/api/runtime/model-jobs")
	case "job":
		if len(args) < 2 {
			return errors.New("usage: labelctl models job <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]))
	case "job-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl models job-logs <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]) + "/logs")
	case "cancel-job":
		if len(args) < 2 {
			return errors.New("usage: labelctl models cancel-job <job_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(args[1])+"/cancel", map[string]string{})
	case "resume-job":
		if len(args) < 2 {
			return errors.New("usage: labelctl models resume-job <job_id>")
		}
		return postJSON(cfg.addr+"/api/runtime/model-jobs/"+url.PathEscape(args[1])+"/resume", map[string]string{})
	default:
		return fmt.Errorf("unknown models command: %s", args[0])
	}
}

func runDeploy(cfg Config, args []string) error {
	if len(args) == 0 || args[0] == "help" {
		return errors.New("usage: labelctl deploy submit -model <id> -target <target> [-version v] [-runtime runtime] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec] | task <task_id> | task-logs <task_id> | follow-task <task_id> | cancel-task <task_id>")
	}
	switch args[0] {
	case "submit":
		fs := flag.NewFlagSet("deploy submit", flag.ExitOnError)
		modelID := fs.String("model", "", "model id")
		version := fs.String("version", "", "model version")
		target := fs.String("target", "", "deployment target")
		runtime := fs.String("runtime", "python-worker", "runtime")
		replicas := fs.Int("replicas", 1, "replicas")
		dryRun := fs.Bool("dry-run", true, "submit as dry-run recipe only")
		resourceClass := fs.String("resource-class", "", "resource class")
		strategy := fs.String("strategy", "dry-run", "deployment strategy")
		canary := fs.Int("canary-percent", 0, "canary percent")
		rollback := fs.String("rollback-policy", "", "rollback policy")
		execSpec := registerExecutionFlags(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*modelID) == "" || strings.TrimSpace(*target) == "" {
			return errors.New("usage: labelctl deploy submit -model <id> -target <target> [-version v] [-runtime runtime] [-exec cmd -exec-arg=arg -exec-cwd dir -exec-env KEY=VALUE -exec-timeout sec]")
		}
		body := compactBody(map[string]any{
			"model_id":        *modelID,
			"model_version":   *version,
			"target":          *target,
			"runtime":         *runtime,
			"replicas":        *replicas,
			"dry_run":         *dryRun,
			"resource_class":  *resourceClass,
			"strategy":        *strategy,
			"canary_percent":  *canary,
			"rollback_policy": *rollback,
		})
		mergeExecutionBody(body, execSpec)
		return postJSON(cfg.addr+"/api/deployments", body)
	case "task", "status":
		if len(args) < 2 {
			return errors.New("usage: labelctl deploy task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	case "task-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl deploy task-logs <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs")
	case "follow-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl deploy follow-task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs/stream")
	case "cancel-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl deploy cancel-task <task_id>")
		}
		return deleteJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]))
	default:
		return fmt.Errorf("unknown deploy command: %s", args[0])
	}
}

func runLogs(cfg Config, args []string) error {
	if len(args) == 0 {
		args = []string{"traces"}
	}
	switch args[0] {
	case "traces", "runtime":
		return getJSON(cfg.addr + "/api/runtime/traces")
	case "audit":
		return getJSON(cfg.addr + "/api/audit-events")
	case "runs":
		return getJSON(cfg.addr + "/api/agent-runs")
	case "jobs":
		return getJSON(cfg.addr + "/api/runtime/model-jobs")
	case "tasks":
		return getJSON(cfg.addr + "/api/tasks")
	case "job", "job-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl logs job <job_id>")
		}
		return getJSON(cfg.addr + "/api/runtime/model-jobs/" + url.PathEscape(args[1]) + "/logs")
	case "task", "task-logs":
		if len(args) < 2 {
			return errors.New("usage: labelctl logs task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs")
	case "follow-task":
		if len(args) < 2 {
			return errors.New("usage: labelctl logs follow-task <task_id>")
		}
		return getJSON(cfg.addr + "/api/tasks/" + url.PathEscape(args[1]) + "/logs/stream")
	case "intake":
		return getJSON(cfg.addr + "/api/runtime/intake/workflows")
	default:
		return fmt.Errorf("unknown logs command: %s", args[0])
	}
}

func runDoctor(cfg Config, args []string) error {
	checks := []struct {
		name string
		path string
	}{
		{name: "health", path: "/healthz"},
		{name: "runtime", path: "/api/runtime/status"},
		{name: "channels", path: "/api/channels"},
		{name: "qq", path: "/api/channels/qq/status"},
		{name: "desktop", path: "/api/desktop/status"},
		{name: "models", path: "/api/models"},
		{name: "datasets", path: "/api/datasets"},
	}
	fmt.Printf("labelctl doctor gateway=%s token=%s\n", strings.TrimRight(cfg.addr, "/"), tokenState(cfg.token))
	failures := 0
	for _, check := range checks {
		status, elapsed, err := probeEndpoint(cfg, check.path)
		if err != nil {
			failures++
			fmt.Printf("  %-10s fail  %s\n", check.name, err)
			continue
		}
		if status >= 400 {
			failures++
			fmt.Printf("  %-10s fail  status=%d elapsed=%s\n", check.name, status, elapsed)
			continue
		}
		fmt.Printf("  %-10s ok    status=%d elapsed=%s\n", check.name, status, elapsed)
	}
	if failures > 0 {
		return fmt.Errorf("doctor found %d failed checks", failures)
	}
	return nil
}

func deleteJSON(rawURL string) error {
	req, err := http.NewRequest(http.MethodDelete, rawURL, nil)
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

func probeEndpoint(cfg Config, path string) (int, time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.addr, "/")+path, nil)
	if err != nil {
		return 0, 0, err
	}
	applyGatewayAuth(req, cfg.token)
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		return 0, elapsed, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, elapsed, nil
}

func compactBody(body map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range body {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				out[key] = typed
			}
		case []string:
			if len(typed) > 0 {
				out[key] = typed
			}
		case int:
			if typed != 0 {
				out[key] = typed
			}
		default:
			out[key] = typed
		}
	}
	return out
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*f = append(*f, value)
	return nil
}

type kvPairsFlag map[string]string

func (f *kvPairsFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*f))
	for key, value := range *f {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func (f *kvPairsFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	key, raw, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("expected KEY=VALUE, got %q", value)
	}
	key = strings.TrimSpace(key)
	raw = strings.TrimSpace(raw)
	if key == "" || raw == "" {
		return fmt.Errorf("expected KEY=VALUE, got %q", value)
	}
	if *f == nil {
		*f = map[string]string{}
	}
	(*f)[key] = raw
	return nil
}

type executionFlags struct {
	command *string
	args    *repeatedStringFlag
	cwd     *string
	env     *kvPairsFlag
	timeout *int
}

func registerExecutionFlags(fs *flag.FlagSet) executionFlags {
	command := fs.String("exec", "", "execution command for non-dry-run worker task")
	args := repeatedStringFlag{}
	fs.Var(&args, "exec-arg", "execution command argument; repeat as needed")
	cwd := fs.String("exec-cwd", "", "working directory for non-dry-run worker task")
	env := kvPairsFlag{}
	fs.Var(&env, "exec-env", "execution environment override KEY=VALUE; repeat as needed")
	timeout := fs.Int("exec-timeout", 0, "execution timeout seconds for non-dry-run worker task")
	return executionFlags{
		command: command,
		args:    &args,
		cwd:     cwd,
		env:     &env,
		timeout: timeout,
	}
}

func mergeExecutionBody(body map[string]any, spec executionFlags) {
	if spec.command == nil || strings.TrimSpace(*spec.command) == "" {
		return
	}
	command := []string{strings.TrimSpace(*spec.command)}
	command = append(command, []string(*spec.args)...)
	body["execution_command"] = command
	if spec.cwd != nil && strings.TrimSpace(*spec.cwd) != "" {
		body["execution_cwd"] = strings.TrimSpace(*spec.cwd)
	}
	if spec.env != nil && len(*spec.env) > 0 {
		body["execution_env"] = map[string]string(*spec.env)
	}
	if spec.timeout != nil && *spec.timeout > 0 {
		body["execution_timeout_seconds"] = *spec.timeout
	}
}

func tokenState(token string) string {
	if strings.TrimSpace(token) == "" {
		return "not-configured"
	}
	return "configured"
}
