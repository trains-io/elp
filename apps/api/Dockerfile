# syntax=docker/dockerfile:1.4
# Build from the elp repository root:
#   docker build -f apps/api/Dockerfile .
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY operators/z21-device/go.mod operators/z21-device/go.sum ./operators/z21-device/
COPY operators/z21-sim-controller/go.mod operators/z21-sim-controller/go.sum ./operators/z21-sim-controller/
COPY packages/events/go.mod ./packages/events/
COPY packages/canpool/go.mod ./packages/canpool/
COPY apps/z21-sim/go.mod apps/z21-sim/go.sum ./apps/z21-sim/

WORKDIR /workspace/apps/api
RUN go mod download

COPY apps/api/ ./
COPY operators/z21-device/ /workspace/operators/z21-device/
COPY operators/z21-sim-controller/api/ /workspace/operators/z21-sim-controller/api/
COPY packages/events/ /workspace/packages/events/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /elp-api .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /elp-api /elp-api

USER 65532:65532

ENTRYPOINT ["/elp-api"]
