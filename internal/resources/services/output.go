package services

import (
	"fmt"
	"io"
	"strings"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, services []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, services)
	case "json":
		return resourcecommon.PrintJSON(w, services)
	case "yaml":
		return resourcecommon.PrintYAML(w, services)
	case "name":
		return printName(w, services)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, services []Info) error {
	if _, err := fmt.Fprintf(w, "Services: %d\n", len(services)); err != nil {
		return err
	}

	for _, service := range services {
		ports := "-"
		if len(service.Ports) > 0 {
			ports = strings.Join(service.Ports, ",")
		}
		if _, err := fmt.Fprintf(
			w,
			"  %s/%s type=%s clusterIP=%s ports=%s\n",
			service.Namespace,
			service.Name,
			service.Type,
			service.ClusterIP,
			ports,
		); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, services []Info) error {
	for _, service := range services {
		name := "service/"
		if strings.TrimSpace(service.Namespace) != "" {
			name += service.Namespace + "/"
		}
		name += service.Name
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}
