# Contributing

This guide covers local development for **elp** — the production home for the Z21
Kubernetes operator and related components.

Run `make` or `make help` from the repository root to list available targets.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Operator module (`operators/z21-device`) |
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
kubectl get pods -n system
kubectl get deploy -A | grep z21
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
kind, and applies `operators/z21-device/config/default` (CRD, RBAC, manager Deployment).

**Sample `Z21Device`** — edit `spec.backend.hardware.host` for your LAN command station,
or use a simulator backend once gateway images are available locally.

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
| `make nats-install` | Install NATS into `default` |
| `make nats-uninstall` | Remove NATS |
| `make dev-infra-up` | `kind-up` + `nats-install` |
| `make dev-infra-down` | `kind-down` |

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
kubectl rollout restart deployment/controller-manager -n system
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
├── deploy/nats/                  # in-cluster NATS for local dev
├── tools/kind/                   # kind config + host.docker.internal setup
└── operators/z21-device/
    ├── api/v1alpha1/             # CRD Go types
    ├── cmd/                      # controller manager entrypoint
    ├── config/                   # kustomize: CRD, RBAC, manager, samples
    ├── internal/controller/      # reconcilers
    └── Dockerfile                # controller manager image
```
