package deployments

import (
	"fmt"
	"io"
	"strings"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, deployments []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, deployments)
	case "json":
		return resourcecommon.PrintJSON(w, deployments)
	case "yaml":
		return resourcecommon.PrintYAML(w, deployments)
	case "name":
		return printName(w, deployments)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, deployments []Info) error {
	if _, err := fmt.Fprintf(w, "Deployments: %d\n", len(deployments)); err != nil {
		return err
	}

	for _, deployment := range deployments {
		if _, err := fmt.Fprintf(
			w,
			"  %s/%s ready=%d/%d updated=%d available=%d\n",
			deployment.Namespace,
			deployment.Name,
			deployment.Ready,
			deployment.Desired,
			deployment.Updated,
			deployment.Available,
		); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, deployments []Info) error {
	for _, deployment := range deployments {
		name := "deployment/"
		if strings.TrimSpace(deployment.Namespace) != "" {
			name += deployment.Namespace + "/"
		}
		name += deployment.Name
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}
