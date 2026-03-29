package pods

import (
	"fmt"
	"io"
	"strings"

	resourcecommon "valley/internal/resources/common"
)

func Print(w io.Writer, pods []Info, opts resourcecommon.QueryOptions) error {
	switch opts.Output {
	case "text":
		return printText(w, pods, opts.Wide)
	case "json":
		return resourcecommon.PrintJSON(w, pods)
	case "yaml":
		return resourcecommon.PrintYAML(w, pods)
	case "name":
		return printName(w, pods)
	default:
		return fmt.Errorf("unsupported format: %s", opts.Output)
	}
}

func printText(w io.Writer, pods []Info, wide bool) error {
	if _, err := fmt.Fprintf(w, "Pods: %d\n", len(pods)); err != nil {
		return err
	}

	if wide {
		if _, err := fmt.Fprintln(w, "NAMESPACE  NAME  PHASE  IP"); err != nil {
			return err
		}
	}

	for _, pod := range pods {
		if wide {
			ip := pod.IP
			if ip == "" {
				ip = "-"
			}
			if _, err := fmt.Fprintf(w, "%s  %s  %s  %s\n", pod.Namespace, pod.Name, pod.Phase, ip); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintf(w, "  %s/%s\n", pod.Namespace, pod.Name); err != nil {
			return err
		}
	}

	return nil
}

func printName(w io.Writer, pods []Info) error {
	for _, pod := range pods {
		name := "pod/"
		if podsNamespaceSet(pod.Namespace) {
			name += pod.Namespace + "/"
		}
		name += pod.Name

		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}

func podsNamespaceSet(namespace string) bool {
	return strings.TrimSpace(namespace) != ""
}
