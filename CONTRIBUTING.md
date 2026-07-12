# Contributing

This guide covers local development for **elp** — the production home for the Z21
Kubernetes operator and related components.

Run `make` or `make help` from the repository root to list available targets.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Operator (`operators/z21-device`) and API (`apps/api`) modules |
| Docker | recent | kind cluster, NATS, controller images |
| kind | recent | Local Kubernetes cluster |
| kubectl | matches kind node | Install CRDs and workloads |
| controller-gen | v0.18.0 | Regenerate CRD, deepcopy, RBAC (`make generate-operator`) |

Install controller-gen:

```bash
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0
```

## Typical local workflow

From the repository root:

```bash
# 1. Cluster + messaging
make dev-infra-up

# 2. Operator (build image, load into kind, install)
make operator-dev-install

# 3. Example device
kubectl apply -f operators/z21-device/config/samples/z21_v1alpha1_z21device.yaml

# 4. Observe
kubectl get z21devices
kubectl get pods -n elp
kubectl get deploy -A | grep z21

# 5. API (optional — HTTP bridge to Z21Device CRs)
make build-api
# In another terminal, port-forward NATS when running the API on the host:
# kubectl port-forward -n default svc/nats 4222:4222
NATS_URL=nats://127.0.0.1:4222 make run-api
curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/namespaces/default/devices
```

Tear down:

```bash
make operator-uninstall   # optional: remove operator + CRDs
make nats-uninstall       # optional: remove NATS only
make dev-infra-down       # delete kind cluster
```

### What each step does

**`make dev-infra-up`** creates a kind cluster named `elp` (override with
`KIND_CLUSTER_NAME`) and installs NATS in the `default` namespace.

- NATS URL: `nats://nats.default.svc.cluster.local:4222`
- The kind node is labelled `trains.io/edge=true` (matches sample `nodeSelector`)
- `host.docker.internal` is mapped for hostNetwork gateways reaching a Z21 on the Docker host

**`make operator-dev-install`** builds `z21-device-controller:local`, loads it into
kind, and applies `operators/z21-device/config/default` (CRD, RBAC, manager
Deployment in the `elp` namespace).

**Sample `Z21Device`** — edit `spec.backend.hardware.host` for your LAN command station,
or use a simulator backend once gateway images are available locally.

## HTTP API

The API module lives at `apps/api/`. It exposes REST endpoints and an SSE device
stream backed by a Kubernetes watch on `Z21Device` resources.

**OpenAPI contract:** `packages/api/openapi/openapi.yaml`

### Build and run

```bash
make test-api
make build-api          # → bin/elp-api
make run-api            # listens on :8080 by default
```

Environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `API_ADDR` | `:8080` | HTTP listen address |
| `NATS_URL` | _(empty)_ | Override NATS URL for control commands |

When the API runs on your host against a kind cluster, device specs still contain
in-cluster NATS URLs (`nats://nats.default.svc.cluster.local:4222`). Set
`NATS_URL=nats://127.0.0.1:4222` and port-forward NATS so broadcast-flag patches
reach gateways:

```bash
kubectl port-forward -n default svc/nats 4222:4222
NATS_URL=nats://127.0.0.1:4222 make run-api
```

### Smoke test

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/namespaces/default/devices
curl -N http://localhost:8080/api/v1/namespaces/default/devices/stream
```

CORS allows `http://localhost:5173` and `:3000` for a future web UI.

### Run in-cluster (kind)

Deploy the API into the cluster so it uses in-cluster kubeconfig and NATS.
`make dev-infra-up` installs MetalLB so the API `LoadBalancer` service gets an
external IP reachable from your host (no port-forward required).

```bash
make dev-infra-up          # kind + MetalLB + NATS
make operator-dev-install  # Z21Device CRD + controller
make api-dev-install       # build elp-api:local, load into kind, apply deploy/api
```

`make api-install` prints the external URL when MetalLB assigns an IP. Generate
elpconfig from the cluster (uses kubectl + the active context):

```bash
elpctl config init
elpctl device list
```

Or preview without writing:

```bash
elpctl config init --dry-run
elpctl config view
```

`elpctl` reads `~/.elp/config` by default (override with `ELPCONFIG` or
`--elpconfig`). `--server` and `ELP_SERVER` still override the config file.

```bash
curl http://<EXTERNAL-IP>:8080/healthz
./bin/elpctl device list
```

If the IP is not shown, check `kubectl get svc elp-api -n elp`. As a fallback you can
still use `make api-port-forward` for `http://localhost:8080`.

Requires NATS (`make nats-install` or `make dev-infra-up`). The Deployment sets
`NATS_URL=nats://nats.default.svc.cluster.local:4222` for control commands.

Remove with `make api-uninstall`.

## Make targets

### Operator

| Target | Description |
|--------|-------------|
| `make test` | Unit tests for the operator module |
| `make generate-operator` | Regenerate deepcopy, CRD, and RBAC from Go types |
| `make build-operator` | Build `bin/z21-device-controller` (host binary) |
| `make operator-image` | Build `z21-device-controller:local` container image |
| `make kind-load-operator` | Load the local image into the kind cluster |
| `make operator-install-crd` | Apply CRDs only |
| `make operator-install` | Apply CRD + RBAC + manager (expects image on the node) |
| `make operator-uninstall` | Remove the full operator install |
| `make operator-dev-install` | Image build + kind load + install |

### Infrastructure

| Target | Description |
|--------|-------------|
| `make kind-up` | Create/configure kind cluster `elp` |
| `make kind-down` | Delete kind cluster |
| `make kind-configure-host` | Re-apply `host.docker.internal` DNS (after `kind-up`) |
| `make metallb-install` | Install MetalLB and configure a kind Docker-network IP pool |
| `make metallb-uninstall` | Remove MetalLB |
| `make nats-install` | Install NATS into `default` |
| `make nats-uninstall` | Remove NATS |
| `make dev-infra-up` | `kind-up` + `metallb-install` + `nats-install` |
| `make dev-infra-down` | `kind-down` |

### API

| Target | Description |
|--------|-------------|
| `make test-api` | Unit tests for `apps/api` |
| `make build-api` | Build `bin/elp-api` |
| `make run-api` | Build (if needed) and run the HTTP API on the host |
| `make api-image` | Build `elp-api:local` container image |
| `make kind-load-api` | Load the local API image into kind |
| `make api-install` | Apply `deploy/api` (Deployment, LoadBalancer Service, RBAC) |
| `make api-uninstall` | Remove the in-cluster API |
| `make api-dev-install` | Image build + kind load + install |
| `make api-port-forward` | Fallback: forward `svc/elp-api` to `localhost:8080` |
| `make test-elpctl` | Unit tests for `apps/elpctl` |
| `make build-elpctl` | Build `bin/elpctl` |

## elpctl

`elpctl` is a kubectl-style CLI for the elp HTTP API.

```bash
make build-elpctl

# against a running API (make run-api, LoadBalancer, or port-forward)
elpctl config init          # writes ~/.elp/config from kubectl + elp-api Service
elpctl device list

elpctl device create basement \
  --address 192.168.0.42:21105 \
  --host-network \
  --node-selector trains.io/edge=true

elpctl device create sim-bench \
  --backend simulator \
  --gateway-image z21-gateway:local

elpctl device list
elpctl device get basement
elpctl device watch basement
elpctl device watch          # all devices in namespace
```

Environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `ELPCONFIG` | `~/.elp/config` | elpconfig file path (`--elpconfig`) |
| `ELP_SERVER` | from elpconfig, else `http://localhost:8080` | API base URL (`--server`) |
| `ELP_NAMESPACE` | from elpconfig, else `default` | Target namespace (`--namespace`) |

### elpconfig

kubeconfig-style file for API server address and namespace:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://172.18.255.200:8080
  name: kind-elp
contexts:
- context:
    cluster: kind-elp
    namespace: default
  name: kind-elp@default
current-context: kind-elp@default
```

`elpctl config init` discovers the `elp-api` LoadBalancer address in the `elp`
namespace via kubectl and writes this file. Cluster/context names follow the
active kubectl context; device namespace defaults to `default`.

Use `-o json` for machine-readable output.

## Developing the operator

The operator module lives at `operators/z21-device/`. It is a standalone Go module
(not a root `go.work` yet).

### Run tests

```bash
make test
# or
cd operators/z21-device && go test ./... -count=1
```

Filter a single test:

```bash
cd operators/z21-device && go test ./internal/controller/... -run TestDesiredGatewayDeployment -v -count=1
```

### Change API types or controllers

1. Edit files under `api/v1alpha1/` and/or `internal/controller/`
2. Regenerate manifests:

   ```bash
   make generate-operator
   ```

3. Run tests and commit both hand-written and generated files

### Reinstall after controller changes

```bash
make operator-dev-install
```

To refresh only the CRD after an API change:

```bash
make operator-install-crd
```

## Developing the API

The API is a standalone Go module at `apps/api/` (with local `replace` directives
for `operators/z21-device` and `packages/events`).

```bash
make test-api
# or
cd apps/api && go test ./... -count=1
```

Requires a kubeconfig pointing at your cluster (e.g. `kind-elp` after
`make dev-infra-up`). The API uses in-cluster config when run inside Kubernetes,
or `~/.kube/config` when run on the host.

```bash
make build-api
NATS_URL=nats://127.0.0.1:4222 make run-api
```

## Checking a Z21Device

After applying a sample:

```bash
kubectl get z21devices
kubectl describe z21device basement
kubectl get deploy,svc -A | grep z21
```

Useful status fields:

- `status.phase` — `Pending`, `Starting`, `Running`, `Failed`, `Stopping`
- `status.conditions[GatewayReady]` — gateway Deployment ready (set by operator)
- `status.conditions[DeviceReachable]` — Z21 reachable (set by gateway, when deployed)

Gateway pods are reconciled in the `elp` namespace (see controller
`--workload-namespace`). Per-device RoleBindings remain in the `Z21Device`
namespace so the gateway can patch device status.

Gateway pods are not functional until a `z21-gateway` image is built and loaded into
kind (the kustomize overlay rewrites `ghcr.io/trains-io/z21-gateway` to
`z21-gateway:local`). That image build will be added to elp in a later phase.

## Troubleshooting

### `kubectl` cannot reach the cluster

```bash
kind export kubeconfig --name elp
kubectl config use-context kind-elp
kubectl cluster-info
```

### `kind-up` times out on WSL2

Docker may be using cgroup v1. Upgrade WSL and enable cgroup v2, then:

```bash
make kind-down && make kind-up
```

See [kind documentation](https://kind.sigs.k8s.io/) for node image overrides:

```bash
KIND_NODE_IMAGE=kindest/node:v1.32.11 make kind-up
```

### Controller pod `ImagePullBackOff`

On kind, the manager image must be loaded locally:

```bash
make operator-image kind-load-operator
kubectl rollout restart deployment/z21-device-controller -n elp
```

Or use the all-in-one target: `make operator-dev-install`.

### Gateway cannot resolve NATS or `host.docker.internal`

Re-run host DNS setup after `kind-up`:

```bash
make kind-configure-host
```

HostNetwork gateway pods use `dnsPolicy: ClusterFirstWithHostNet` so in-cluster
service names such as `nats.default.svc.cluster.local` resolve via CoreDNS.

## Repository layout

```
elp/
├── Makefile                      # dev targets (see make help)
├── deploy/
│   ├── api/                      # in-cluster API (Deployment, RBAC, Service)
│   └── nats/                     # in-cluster NATS for local dev
├── packages/
│   ├── api/openapi/              # OpenAPI contract for the HTTP API
│   └── events/                   # shared NATS event/control types
├── tools/kind/                   # kind config + host.docker.internal setup
├── apps/
│   ├── api/                      # HTTP API (REST + SSE device stream)
│   │   └── Dockerfile
│   └── elpctl/                   # kubectl-style CLI for the API
└── operators/z21-device/
    ├── api/v1alpha1/             # CRD Go types
    ├── cmd/                      # controller manager entrypoint
    ├── config/                   # kustomize: CRD, RBAC, manager, samples
    ├── internal/controller/      # reconcilers
    └── Dockerfile                # controller manager image
```
