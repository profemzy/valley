package namespaces

import (
	"context"
	"io"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

type getHandler struct{}

func (getHandler) Names() []string {
	return []string{"namespaces", "namespace", "ns"}
}

func (getHandler) Get(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
	namespaces, err := List(ctx, rt.Typed, opts)
	if err != nil {
		return err
	}

	return Print(w, namespaces, opts)
}

var GetHandler resourcecommon.GetHandler = getHandler{}
