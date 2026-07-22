.PHONY: help build run run-trace test tidy proto docker-build docker-run docker-stop test-integration

.DEFAULT_GOAL := help

BIN := bin/z21-sim
ADDR ?= 127.0.0.1:21105
GRPC_ADDR ?= 127.0.0.1:50051
IMAGE ?= ghcr.io/trains-io/z21-sim
TAG ?= local
CONTAINER ?= z21-sim
export Z21_TESTSERVER_IMAGE ?= $(IMAGE):$(TAG)

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

proto: ## Regenerate gRPC code from api/control/v1/control.proto
	protoc --go_out=. --go_opt=module=github.com/trains-io/elp/apps/z21-sim \
		--go-grpc_out=. --go-grpc_opt=module=github.com/trains-io/elp/apps/z21-sim \
		api/control/v1/control.proto

build: ## Compile binary to bin/
	go build -o $(BIN) ./cmd/z21-sim

run: build ## Run the simulator locally
	$(BIN) -addr $(ADDR) -grpc-addr $(GRPC_ADDR)

run-trace: build ## Run with LAN and gRPC trace dumps on stderr
	$(BIN) -addr $(ADDR) -grpc-addr $(GRPC_ADDR) -trace

test: ## Run unit tests
	go test ./... -count=1

test-integration: docker-build ## Run Docker integration tests
	go test -tags=integration ./integration/... -count=1 -timeout=10m

tidy: ## Tidy module dependencies
	go mod tidy

docker-build: ## Build the Docker image
	docker build -t $(IMAGE):$(TAG) .

docker-run: ## Start the simulator container detached
	docker run --rm -d --name $(CONTAINER) -p 21105:21105/udp -p 50051:50051/tcp $(IMAGE):$(TAG)

docker-stop: ## Stop and remove the simulator container
	@docker rm -f $(CONTAINER) >/dev/null 2>&1 && echo "Stopped $(CONTAINER)" || echo "No container named $(CONTAINER)"
