package output

import (
	"encoding/json"
	"fmt"
	"io"

	"valley/internal/app"
)

func PrintPods(w io.Writer, pods []app.PodInfo, format string) error {
	switch format {
	case "text":
		return printPodsText(w, pods)
	case "json":
		return printPodsJSON(w, pods)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printPodsText(w io.Writer, pods []app.PodInfo) error {
	if _, err := fmt.Fprintf(w, "Pods: %d\n", len(pods)); err != nil {
		return err
	}

	for _, pod := range pods {
		if _, err := fmt.Fprintf(w, "  %s/%s\n", pod.Namespace, pod.Name); err != nil {
			return err
		}
	}

	return nil
}

func printPodsJSON(w io.Writer, pods []app.PodInfo) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pods)
}
