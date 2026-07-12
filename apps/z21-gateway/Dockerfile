# syntax=docker/dockerfile:1.4
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

COPY apps/z21-gateway/go.mod apps/z21-gateway/go.sum ./apps/z21-gateway/
COPY operators/z21-device/go.mod operators/z21-device/go.sum ./operators/z21-device/
COPY packages/events/go.mod ./packages/events/
COPY --from=z21go . ./z21.go/

COPY apps/z21-gateway/ ./apps/z21-gateway/
COPY operators/z21-device/ ./operators/z21-device/
COPY packages/events/ ./packages/events/

WORKDIR /workspace/apps/z21-gateway
RUN go mod edit -dropreplace=github.com/trains-io/z21.go && \
	go mod edit -replace=github.com/trains-io/z21.go=/workspace/z21.go
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /z21-gateway .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /z21-gateway /z21-gateway

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/z21-gateway"]
