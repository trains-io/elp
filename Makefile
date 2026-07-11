.DEFAULT_GOAL := help

.PHONY: help test generate-operator build-operator clean \
	operator-image kind-load-operator operator-install-crd operator-install operator-uninstall operator-dev-install \
	kind-up kind-down kind-configure-host nats-install nats-uninstall dev-infra-up dev-infra-down \
	test-api build-api run-api api-image kind-load-api api-install api-uninstall api-dev-install api-port-forward \
	test-elpctl build-elpctl

KIND_CLUSTER_NAME ?= elp
KIND_CONFIG = tools/kind/kind-config.yaml
KIND_NODE = $(KIND_CLUSTER_NAME)-control-plane
OPERATOR_DIR = operators/z21-device
OPERATOR_CRD_KUSTOMIZE = $(OPERATOR_DIR)/config/crd
OPERATOR_KUSTOMIZE = $(OPERATOR_DIR)/config/default
OPERATOR_IMAGE = z21-device-controller:local
API_DIR = apps/api
API_BIN = bin/elp-api
API_KUSTOMIZE = deploy/api
API_IMAGE = elp-api:local
ELPCTL_DIR = apps/elpctl
ELPCTL_BIN = bin/elpctl
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

operator-image: ## Build the controller container image for local kind
	docker build -t $(OPERATOR_IMAGE) -f $(OPERATOR_DIR)/Dockerfile $(OPERATOR_DIR)

kind-load-operator: ## Load the local controller image into the kind cluster
	@if ! kind get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER_NAME)'; then \
		echo "kind cluster '$(KIND_CLUSTER_NAME)' not found; run make kind-up first"; exit 1; \
	fi
	kind load docker-image $(OPERATOR_IMAGE) --name $(KIND_CLUSTER_NAME)

operator-install-crd: ## Install Z21Device CRDs only
	kubectl apply -k $(OPERATOR_CRD_KUSTOMIZE)

operator-install: ## Install CRDs, RBAC, and controller manager
	kubectl apply -k $(OPERATOR_KUSTOMIZE)
	kubectl rollout status deployment/controller-manager -n system --timeout=120s

operator-uninstall: ## Remove controller manager, RBAC, and CRDs
	kubectl delete -k $(OPERATOR_KUSTOMIZE) --ignore-not-found

operator-dev-install: operator-image kind-load-operator operator-install ## Build, load, and install operator on kind

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

##@ API

test-api: ## Run API unit tests
	cd $(API_DIR) && go test ./... -count=1

build-api: ## Build bin/elp-api
	mkdir -p bin
	cd $(API_DIR) && go build -o ../../$(API_BIN) .

run-api: build-api ## Run HTTP API on :8080 (see CONTRIBUTING.md for NATS_URL with kind)
	$(API_BIN)

api-image: ## Build the API container image for local kind
	docker build -t $(API_IMAGE) -f $(API_DIR)/Dockerfile .

kind-load-api: ## Load the local API image into the kind cluster
	@if ! kind get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER_NAME)'; then \
		echo "kind cluster '$(KIND_CLUSTER_NAME)' not found; run make kind-up first"; exit 1; \
	fi
	kind load docker-image $(API_IMAGE) --name $(KIND_CLUSTER_NAME)

api-install: ## Install API Deployment, Service, and RBAC into the cluster
	kubectl apply -k $(API_KUSTOMIZE)
	kubectl rollout status deployment/elp-api -n default --timeout=120s
	@echo "In-cluster URL: http://elp-api.default.svc.cluster.local:8080"
	@echo "Host access: make api-port-forward"

api-uninstall: ## Remove in-cluster API Deployment, Service, and RBAC
	kubectl delete -k $(API_KUSTOMIZE) --ignore-not-found

api-dev-install: api-image kind-load-api api-install ## Build, load, and install API on kind

api-port-forward: ## Forward in-cluster API to localhost:8080
	kubectl port-forward -n default svc/elp-api 8080:8080

test-elpctl: ## Run elpctl unit tests
	cd $(ELPCTL_DIR) && go test ./... -count=1

build-elpctl: ## Build bin/elpctl
	mkdir -p bin
	cd $(ELPCTL_DIR) && go build -o ../../$(ELPCTL_BIN) .
