# syntax=docker/dockerfile:1.7

FROM golang:1.26 AS build

WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH=amd64

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/valley ./cmd/valley

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=build /out/valley /valley

USER nonroot:nonroot

ENTRYPOINT ["/valley"]
