.DEFAULT_GOAL := help

.PHONY: help test generate-operator build-operator clean \
	kind-up kind-down kind-configure-host nats-install nats-uninstall dev-infra-up dev-infra-down

KIND_CLUSTER_NAME ?= elp
KIND_CONFIG = tools/kind/kind-config.yaml
KIND_NODE = $(KIND_CLUSTER_NAME)-control-plane
# Pin stable k8s; override to match your kind release notes if needed.
KIND_NODE_IMAGE ?= kindest/node:v1.32.11@sha256:5fc52d52a7b9574015299724bd68f183702956aa4a2116ae75a63cb574b35af8

##@ General

help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ Operator

test: ## Run operator unit tests
	cd operators/z21-device && go test ./... -count=1

generate-operator: ## Regenerate CRD, deepcopy, and RBAC from API + controllers
	cd operators/z21-device && \
	controller-gen object paths="./api/..." output:object:artifacts:config=api/v1alpha1 && \
	controller-gen crd rbac:roleName=manager-role \
		paths="./api/...;./internal/..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:dir=config/rbac

build-operator: ## Build the z21-device operator
	mkdir -p bin
	cd operators/z21-device && go build -o ../../bin/z21-device-controller ./cmd

clean: ## Remove built binaries
	rm -rf bin/

##@ Infrastructure

kind-up: ## Create local kind cluster (requires kind and kubectl)
	@if kind get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER_NAME)'; then \
		echo "kind cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		echo "Creating kind cluster '$(KIND_CLUSTER_NAME)' with image $(KIND_NODE_IMAGE)"; \
		kind create cluster --name $(KIND_CLUSTER_NAME) --image $(KIND_NODE_IMAGE) --config $(KIND_CONFIG) --wait 5m || \
		( echo ""; \
		  echo "kind create failed. On WSL2 this is often caused by cgroup v1 — use cgroup v2 and retry."; \
		  echo "Try: make kind-down && make kind-up"; \
		  exit 1 ); \
	fi
	$(MAKE) kind-configure-host
	kind export kubeconfig --name $(KIND_CLUSTER_NAME)
	kubectl config use-context kind-$(KIND_CLUSTER_NAME)
	@echo "kubectl context: kind-$(KIND_CLUSTER_NAME)"
	kubectl cluster-info

kind-configure-host: ## Map host.docker.internal for hostNetwork gateways (kind/WSL)
	@if ! docker inspect $(KIND_NODE) >/dev/null 2>&1; then \
		echo "kind node '$(KIND_NODE)' not found; run make kind-up first"; exit 1; \
	fi
	@HOST_IP=$$(docker exec $(KIND_NODE) ip route | awk '/default/ {print $$3}'); \
		export HOST_IP; \
		bash tools/kind/configure-host.sh $(KIND_NODE)

kind-down: ## Delete local kind cluster
	@if kind get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER_NAME)'; then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
	else \
		echo "kind cluster '$(KIND_CLUSTER_NAME)' does not exist"; \
	fi

nats-install: ## Install NATS into the default namespace
	kubectl apply -k deploy/nats
	kubectl rollout status deployment/nats -n default --timeout=120s
	@echo "NATS client URL: nats://nats.default.svc.cluster.local:4222"

nats-uninstall: ## Remove NATS from the cluster
	kubectl delete -k deploy/nats --ignore-not-found

dev-infra-up: kind-up nats-install ## Bring up kind cluster and install NATS

dev-infra-down: kind-down ## Tear down kind cluster
