ARG OS=linux
ARG ARCH=amd64
# Build the manager binary
FROM golang:1.23@sha256:60deed95d3888cc5e4d9ff8a10c54e5edc008c6ae3fba6187be6fb592e19e8c0 as builder
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
FROM --platform=linux/${ARCH} alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

WORKDIR /
COPY --from=builder /workspace/addon-manager /

ENTRYPOINT ["/addon-manager"]
