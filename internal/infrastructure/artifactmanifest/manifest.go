package artifactmanifest

import "strings"

const SchemaVersionV1 = "artifact-manifest/v1"

type Entry struct {
	Name     string
	URI      string
	Kind     string
	Metadata map[string]string
}

type Summary struct {
	ArtifactCount       int              `json:"artifact_count"`
	RoleCounts          map[string]int   `json:"role_counts,omitempty"`
	KindCounts          map[string]int   `json:"kind_counts,omitempty"`
	ExecutionModeCounts map[string]int   `json:"execution_mode_counts,omitempty"`
	PrimaryArtifact     *PrimaryArtifact `json:"primary_artifact,omitempty"`
}

type PrimaryArtifact struct {
	Name          string `json:"name"`
	URI           string `json:"uri"`
	Kind          string `json:"kind,omitempty"`
	Role          string `json:"role,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
}

func BuildSummary(entries []Entry) Summary {
	summary := Summary{ArtifactCount: len(entries)}
	if len(entries) == 0 {
		return summary
	}

	roleCounts := map[string]int{}
	kindCounts := map[string]int{}
	executionModeCounts := map[string]int{}
	bestIndex := 0
	bestScore := artifactPriority(entries[0])

	for i, entry := range entries {
		if role := Role(entry); role != "" {
			roleCounts[role]++
		}
		if kind := strings.TrimSpace(entry.Kind); kind != "" {
			kindCounts[kind]++
		}
		if mode := strings.TrimSpace(entry.Metadata["execution_mode"]); mode != "" {
			executionModeCounts[mode]++
		}
		score := artifactPriority(entry)
		if score > bestScore {
			bestIndex = i
			bestScore = score
		}
	}

	if len(roleCounts) > 0 {
		summary.RoleCounts = roleCounts
	}
	if len(kindCounts) > 0 {
		summary.KindCounts = kindCounts
	}
	if len(executionModeCounts) > 0 {
		summary.ExecutionModeCounts = executionModeCounts
	}
	summary.PrimaryArtifact = primaryArtifact(entries[bestIndex])
	return summary
}

func Role(entry Entry) string {
	if entry.Metadata != nil {
		if role := strings.TrimSpace(entry.Metadata["role"]); role != "" {
			return role
		}
	}
	name := strings.ToLower(strings.TrimSpace(entry.Name))
	kind := strings.ToLower(strings.TrimSpace(entry.Kind))
	switch {
	case strings.Contains(kind, ".result") || name == "result":
		return "result"
	case strings.Contains(kind, ".recipe_report") || name == "recipe_report" || name == "report":
		return "recipe_report"
	case strings.Contains(kind, ".recipe_spec") || name == "recipe_spec" || name == "spec":
		return "recipe_spec"
	case strings.Contains(kind, ".plan") || name == "plan":
		return "plan"
	case strings.Contains(kind, ".request") || name == "request":
		return "request"
	case strings.Contains(kind, "manifest") || name == "manifest":
		return "manifest"
	default:
		return ""
	}
}

func primaryArtifact(entry Entry) *PrimaryArtifact {
	if strings.TrimSpace(entry.Name) == "" && strings.TrimSpace(entry.URI) == "" && strings.TrimSpace(entry.Kind) == "" {
		return nil
	}
	out := &PrimaryArtifact{
		Name: strings.TrimSpace(entry.Name),
		URI:  strings.TrimSpace(entry.URI),
		Kind: strings.TrimSpace(entry.Kind),
		Role: Role(entry),
	}
	if entry.Metadata != nil {
		out.ExecutionMode = strings.TrimSpace(entry.Metadata["execution_mode"])
	}
	return out
}

func artifactPriority(entry Entry) int {
	role := Role(entry)
	switch role {
	case "result":
		return 600
	case "recipe_report":
		return 500
	case "recipe_spec":
		return 450
	case "plan":
		return 400
	case "request":
		return 300
	case "manifest":
		return 200
	}

	kind := strings.ToLower(strings.TrimSpace(entry.Kind))
	switch {
	case strings.Contains(kind, ".result"):
		return 550
	case strings.Contains(kind, ".report"):
		return 480
	case strings.Contains(kind, ".spec"):
		return 430
	case strings.Contains(kind, ".plan"):
		return 380
	case strings.Contains(kind, ".request"):
		return 280
	case strings.Contains(kind, "manifest"):
		return 180
	}
	if strings.TrimSpace(entry.URI) != "" {
		return 100
	}
	return 0
}
