# syntax=docker/dockerfile:1.4
# Build from the elp repository root:
#   docker build -f operators/z21-device/Dockerfile .
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

COPY operators/z21-device/go.mod operators/z21-device/go.sum ./operators/z21-device/
COPY packages/canpool/go.mod ./packages/canpool/

COPY operators/z21-device/ ./operators/z21-device/
COPY packages/canpool/ ./packages/canpool/

WORKDIR /workspace/operators/z21-device
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /manager ./cmd

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
