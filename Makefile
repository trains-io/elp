.PHONY: help test generate-operator build-operator clean

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

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
	cd operators/z21-device && go build -o ../../bin/z21-device-controller ./cmd

clean: ## Remove built binaries
	rm -f bin/*
