# Build the manager binary
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder
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

ARG API_GROUP_NAME=cluster.core.oam.dev

# Build
RUN CGO_ENABLED=0 \
    GOOS=${OS} \
    GOARCH=${ARCH} \
    go build \
        -ldflags="-X 'github.com/oam-dev/cluster-gateway/pkg/config.MetaApiGroupName=${API_GROUP_NAME}'" \
        -o apiserver \
        cmd/apiserver/main.go

RUN CGO_ENABLED=0 \
    GOOS=${OS} \
    GOARCH=${ARCH} \
    go build \
        -o patch \
        hack/patch/main.go

# Pinned by digest: the runtime's CA bundle is what the gateway trusts when it connects to managed
# clusters, so a rebuild mustn't pick up whatever alpine:3 is that day. 3.24.4,
# ca-certificates-bundle 20261020-r0 (the eMudhra CA fix).
FROM alpine:3.24@sha256:3f4d6b32e26703c6082839234f43769d5046b3ac7c910033911d1e71d8cd630a

WORKDIR /
COPY --from=builder /workspace/apiserver /
COPY --from=builder /workspace/patch /

ENTRYPOINT ["/apiserver"]
