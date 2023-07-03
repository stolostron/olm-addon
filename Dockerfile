# syntax=docker/dockerfile:1.4

# Build the addon controller binary
FROM --platform=${BUILDPLATFORM} registry.ci.openshift.org/stolostron/builder:go1.19-linux AS builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
USER 0

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod,z \
    go mod download

# Copy the source
COPY main.go .
COPY pkg/ pkg/
COPY manifests/ manifests/

# Build
# We don't vendor modules. Enforce that behavior
ENV GOFLAGS=-mod=readonly
RUN --mount=type=cache,target=/root/.cache/go-build,z \
    --mount=type=cache,target=/go/pkg/mod,z \
    CGO_ENABLED=0 go build -a -o olm-addon-controller

# Use UBI minimal as base image to package the manager binary
FROM registry.access.redhat.com/ubi8/ubi-minimal

RUN microdnf update && \
    microdnf clean all

WORKDIR /
COPY --from=builder /workspace/olm-addon-controller .

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
RUN mkdir -p /data && chown 65532:65532 /data

USER 65532:65532
WORKDIR /data
VOLUME /data

ENTRYPOINT ["/olm-addon-controller"]

