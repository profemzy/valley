package events

import (
	"fmt"
	"io"
	"strings"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, events []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, events)
	case "json":
		return resourcecommon.PrintJSON(w, events)
	case "yaml":
		return resourcecommon.PrintYAML(w, events)
	case "name":
		return printName(w, events)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, events []Info) error {
	if _, err := fmt.Fprintf(w, "Events: %d\n", len(events)); err != nil {
		return err
	}

	for _, event := range events {
		msg := strings.TrimSpace(event.Message)
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}

		if _, err := fmt.Fprintf(
			w,
			"  %s/%s type=%s reason=%s object=%s count=%d msg=%q\n",
			event.Namespace,
			event.Name,
			event.Type,
			event.Reason,
			event.Object,
			event.Count,
			msg,
		); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, events []Info) error {
	for _, event := range events {
		name := "event/"
		if strings.TrimSpace(event.Namespace) != "" {
			name += event.Namespace + "/"
		}
		name += event.Name
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}
