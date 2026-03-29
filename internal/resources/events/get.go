package events

import (
	"context"
	"io"

	"valley/internal/kube"
	resourcecommon "valley/internal/resources/common"
)

type getHandler struct{}

func (getHandler) Names() []string {
	return []string{"events", "event", "ev"}
}

func (getHandler) Get(ctx context.Context, rt *kube.Runtime, opts resourcecommon.QueryOptions, w io.Writer) error {
	events, err := List(ctx, rt.Typed, opts)
	if err != nil {
		return err
	}

	return Print(w, events, opts)
}

var GetHandler resourcecommon.GetHandler = getHandler{}
