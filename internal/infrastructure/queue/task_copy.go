package queue

import "github.com/qsyy0921/automated_training_model/internal/domain/workflow"

func cloneTask(task workflow.Task) workflow.Task {
	task.Payload = copyStringMap(task.Payload)
	task.Metadata = copyStringMap(task.Metadata)
	task.Artifacts = copyTaskArtifacts(task.Artifacts)
	task.Logs = copyTaskLogs(task.Logs)
	if task.WorkerHeartbeat != nil {
		hb := *task.WorkerHeartbeat
		task.WorkerHeartbeat = &hb
	}
	if task.StartedAt != nil {
		started := *task.StartedAt
		task.StartedAt = &started
	}
	if task.FinishedAt != nil {
		finished := *task.FinishedAt
		task.FinishedAt = &finished
	}
	return task
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func copyTaskArtifacts(artifacts []workflow.TaskArtifact) []workflow.TaskArtifact {
	if len(artifacts) == 0 {
		return nil
	}
	out := make([]workflow.TaskArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		copied := artifact
		copied.Metadata = copyStringMap(artifact.Metadata)
		out = append(out, copied)
	}
	return out
}

func copyTaskLogs(logs []workflow.TaskLog) []workflow.TaskLog {
	if len(logs) == 0 {
		return nil
	}
	out := make([]workflow.TaskLog, len(logs))
	copy(out, logs)
	return out
}
