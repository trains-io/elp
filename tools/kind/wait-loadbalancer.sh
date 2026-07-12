#!/usr/bin/env bash
# Wait until a LoadBalancer Service has an external IP assigned.
set -euo pipefail

SVC="${1:?service name required}"
NS="${2:-default}"
TIMEOUT="${3:-120}"

deadline=$((SECONDS + TIMEOUT))
while [ "$SECONDS" -lt "$deadline" ]; do
	IP=$(kubectl get svc "$SVC" -n "$NS" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
	if [ -n "$IP" ]; then
		echo "$IP"
		exit 0
	fi
	sleep 2
done

echo "timed out waiting for LoadBalancer IP on svc/${SVC} in namespace ${NS}" >&2
exit 1
