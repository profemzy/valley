package nodes

import (
	"fmt"
	"io"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, nodes []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, nodes)
	case "json":
		return resourcecommon.PrintJSON(w, nodes)
	case "yaml":
		return resourcecommon.PrintYAML(w, nodes)
	case "name":
		return printName(w, nodes)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, nodes []Info) error {
	if _, err := fmt.Fprintf(w, "Nodes: %d\n", len(nodes)); err != nil {
		return err
	}

	for _, node := range nodes {
		ready := "False"
		if node.Ready {
			ready = "True"
		}
		if _, err := fmt.Fprintf(
			w,
			"  %s ready=%s roles=%s version=%s internalIP=%s\n",
			node.Name,
			ready,
			node.Roles,
			node.Version,
			node.InternalIP,
		); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, nodes []Info) error {
	for _, node := range nodes {
		if _, err := fmt.Fprintf(w, "node/%s\n", node.Name); err != nil {
			return err
		}
	}
	return nil
}
