package namespaces

import (
	"fmt"
	"io"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, namespaces []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, namespaces)
	case "json":
		return resourcecommon.PrintJSON(w, namespaces)
	case "yaml":
		return resourcecommon.PrintYAML(w, namespaces)
	case "name":
		return printName(w, namespaces)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, namespaces []Info) error {
	if _, err := fmt.Fprintf(w, "Namespaces: %d\n", len(namespaces)); err != nil {
		return err
	}

	for _, namespace := range namespaces {
		if _, err := fmt.Fprintf(w, "  %s status=%s\n", namespace.Name, namespace.Status); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, namespaces []Info) error {
	for _, namespace := range namespaces {
		if _, err := fmt.Fprintf(w, "namespace/%s\n", namespace.Name); err != nil {
			return err
		}
	}
	return nil
}
