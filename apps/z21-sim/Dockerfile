# syntax=docker/dockerfile:1.4
# Headless Z21 LAN simulator (Go). UDP API on 21105; gRPC control on 50051.
FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod edit -dropreplace=github.com/trains-io/z21.go
RUN go mod download
COPY api ./api
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /z21-sim ./cmd/z21-sim

FROM debian:bookworm-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /z21-sim /usr/local/bin/z21-sim

EXPOSE 21105/udp
EXPOSE 50051/tcp

ENTRYPOINT ["/usr/local/bin/z21-sim"]
CMD ["-addr", "0.0.0.0:21105", "-grpc-addr", "0.0.0.0:50051"]
