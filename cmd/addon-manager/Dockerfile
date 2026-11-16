ARG OS=linux
ARG ARCH=amd64
# Build the manager binary
FROM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2 AS builder
ARG OS
ARG ARCH

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY hack/ hack/

# Build
RUN CGO_ENABLED=0 \
    GOOS=${OS} \
    GOARCH=${ARCH} \
    GO111MODULE=on \
    go build \
        -a -o addon-manager \
        cmd/addon-manager/main.go

# Pinned by digest (3.24.4, ca-certificates-bundle 20261020-r0, the eMudhra CA fix); multiarch/alpine
# v3.13 is end of life. The image is built per architecture (make ocm-addon-manager ARCH=...), often
# on another architecture's host: --platform picks ARCH's variant of the multi-arch base.
ARG ARCH
FROM --platform=linux/${ARCH} alpine:3.24@sha256:3f4d6b32e26703c6082839234f43769d5046b3ac7c910033911d1e71d8cd630a

WORKDIR /
COPY --from=builder /workspace/addon-manager /

ENTRYPOINT ["/addon-manager"]
