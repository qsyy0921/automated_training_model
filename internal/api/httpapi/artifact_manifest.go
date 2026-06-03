package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func artifactManifestResponseFromPath(path string) (map[string]any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("artifact manifest not found")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	return map[string]any{
		"path":     path,
		"manifest": manifest,
	}, nil
}
