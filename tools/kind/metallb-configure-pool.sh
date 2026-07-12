#!/usr/bin/env bash
# Configure MetalLB L2 pool from the kind Docker network subnet.
set -euo pipefail

NETWORK_NAME="${KIND_NETWORK_NAME:-kind}"

if ! docker network inspect "$NETWORK_NAME" >/dev/null 2>&1; then
	echo "docker network '$NETWORK_NAME' not found; is kind running?" >&2
	exit 1
fi

SUBNET=$(docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' "$NETWORK_NAME")
PREFIX=${SUBNET%/*}
IFS='.' read -r o1 o2 _ _ <<<"$PREFIX"

POOL="${o1}.${o2}.255.200-${o1}.${o2}.255.250"

echo "Configuring MetalLB pool ${POOL} (docker network ${NETWORK_NAME}, subnet ${SUBNET})"

kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - ${POOL}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default
EOF

echo "MetalLB pool ready: ${POOL}"
