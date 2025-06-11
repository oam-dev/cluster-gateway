# Build the manager binary
FROM golang:1.23.8-alpine as builder
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

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3

WORKDIR /
COPY --from=builder /workspace/apiserver /
COPY --from=builder /workspace/patch /

ENTRYPOINT ["/apiserver"]
