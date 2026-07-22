# syntax=docker/dockerfile:1.4
# Build from the elp repository root:
#   docker build -f operators/z21-sim-controller/Dockerfile .
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

COPY operators/z21-sim-controller/go.mod operators/z21-sim-controller/go.sum ./operators/z21-sim-controller/
COPY operators/z21-device/go.mod operators/z21-device/go.sum ./operators/z21-device/
COPY packages/canpool/go.mod ./packages/canpool/
COPY apps/z21-sim/go.mod apps/z21-sim/go.sum ./apps/z21-sim/

COPY operators/z21-sim-controller/ ./operators/z21-sim-controller/
COPY operators/z21-device/api/ ./operators/z21-device/api/
COPY packages/canpool/ ./packages/canpool/
COPY apps/z21-sim/api/ ./apps/z21-sim/api/

WORKDIR /workspace/operators/z21-sim-controller
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /manager ./cmd

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
