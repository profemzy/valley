package deployments

import (
	"fmt"
	"io"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, deployments []Info, format string) error {
	switch format {
	case "text":
		return printText(w, deployments)
	case "json":
		return resourcecommon.PrintJSON(w, deployments)
	default:
		return fmt.Errorf("unsupported format: %s", format)
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
