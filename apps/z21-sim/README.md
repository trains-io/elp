# z21-sim

Go implementation of a [Z21 LAN protocol](https://www.z21.eu) test server for Elevant simulator backends. Replaces the legacy C++ [z21-sim](https://github.com/trains-io/z21-sim) image over time.

- **UDP 21105** — Z21 LAN API (gateway connects here)
- **TCP 50051** — gRPC control plane for a future simulation controller sidecar

Wire formats come from [z21.go](https://github.com/trains-io/z21.go).

## Development

From this directory:

```bash
make test
make run
make docker-build
```

From the elp repository root:

```bash
make test-z21-sim
make build-z21-sim
```

## z21.go integration tests

```bash
export Z21_TESTSERVER_IMAGE=ghcr.io/trains-io/z21-sim:local
make -C apps/z21-sim docker-build TAG=local
make -C ../z21.go test-integration
```

Or point at this Dockerfile context:

```bash
export Z21_TESTSERVER_DOCKERFILE=/path/to/elp/apps/z21-sim
```

## gRPC control API

See [`api/control/v1/control.proto`](api/control/v1/control.proto). Regenerate stubs with `make proto` (requires `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`).
