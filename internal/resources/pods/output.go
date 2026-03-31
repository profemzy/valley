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
	if len(pods) == 0 {
		return nil
	}

	// Build rows: namespace/name + semantic status (+ IP if wide)
	type row struct {
		ref    string
		status string
		ip     string
	}
	rows := make([]row, len(pods))
	maxRef := len("NAME")
	maxStatus := len("STATUS")
	for i, pod := range pods {
		ref := pod.Namespace + "/" + pod.Name
		status := pod.SemanticStatus()
		if len(ref) > maxRef {
			maxRef = len(ref)
		}
		if len(status) > maxStatus {
			maxStatus = len(status)
		}
		ip := pod.IP
		if ip == "" {
			ip = "-"
		}
		rows[i] = row{ref: ref, status: status, ip: ip}
	}

	// Header
	if wide {
		if _, err := fmt.Fprintf(w, "%-*s  %-*s  %s\n", maxRef, "NAME", maxStatus, "STATUS", "IP"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", maxRef, "NAME", "STATUS"); err != nil {
			return err
		}
	}

	// Rows
	for _, r := range rows {
		if wide {
			if _, err := fmt.Fprintf(w, "%-*s  %-*s  %s\n", maxRef, r.ref, maxStatus, r.status, r.ip); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%-*s  %s\n", maxRef, r.ref, r.status); err != nil {
				return err
			}
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
