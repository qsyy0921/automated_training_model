package taxonomyrepo

import (
	"encoding/json"
	"os"

	"github.com/qsyy0921/automated_training_model/internal/domain/taxonomy"
)

func Load(path string) (taxonomy.Taxonomy, error) {
	if path == "" {
		return taxonomy.Default(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return taxonomy.Default(), nil
		}
		return taxonomy.Taxonomy{}, err
	}
	var t taxonomy.Taxonomy
	if err := json.Unmarshal(raw, &t); err != nil {
		return taxonomy.Taxonomy{}, err
	}
	return t.FillDefaults(), nil
}
