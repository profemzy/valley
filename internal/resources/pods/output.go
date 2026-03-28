package pods

import (
	"encoding/json"
	"fmt"
	"io"
)

func Print(w io.Writer, pods []Info, format string) error {
	switch format {
	case "text":
		return printText(w, pods)
	case "json":
		return printJSON(w, pods)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printText(w io.Writer, pods []Info) error {
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

func printJSON(w io.Writer, pods []Info) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pods)
}
