package pods

import (
	"fmt"
	"io"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, pods []Info, format string) error {
	switch format {
	case "text":
		return printText(w, pods)
	case "json":
		return resourcecommon.PrintJSON(w, pods)
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
