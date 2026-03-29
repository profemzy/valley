package common

import (
	"encoding/json"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func PrintYAML(w io.Writer, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	_, err = w.Write(data)
	return err
}
