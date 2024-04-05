ARG builder_image
ARG base_image
ARG target_arch

# Build the manager binary
FROM ${builder_image} as builder
ARG target_arch

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
COPY ./ ./

ARG package=.
ARG gcflags
ARG ldflags
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${target_arch} \
    go build -trimpath \
    -ldflags="${ldflags}"  \
    -gcflags="${gcflags}" \
    -o manager ${package}

FROM --platform=linux/${target_arch} ${base_image}
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
