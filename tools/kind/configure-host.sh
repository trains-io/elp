#!/usr/bin/env bash
# Configure kind node + cluster DNS so hostNetwork gateways can reach:
# - in-cluster NATS (*.svc.cluster.local) via CoreDNS
# - bench Z21 on the Docker host via host.docker.internal
set -euo pipefail

KIND_NODE="${1:?kind node container name required}"
HOST_IP="$(docker exec "$KIND_NODE" ip route | awk '/default/ {print $3}')"

echo "Host gateway IP: $HOST_IP"

docker exec "$KIND_NODE" sh -c \
  'grep -q "[[:space:]]host\.docker\.internal$" /etc/hosts || echo "'"$HOST_IP"' host.docker.internal" >> /etc/hosts'
echo "Configured /etc/hosts on $KIND_NODE"

if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "Cluster not ready; skipping CoreDNS patch (re-run after kind-up)"
  exit 0
fi

if kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}' | grep -q 'host.docker.internal'; then
  echo "CoreDNS already maps host.docker.internal"
else
  kubectl get configmap coredns -n kube-system -o json | HOST_IP="$HOST_IP" python3 -c "
import json, os, sys
host_ip = os.environ['HOST_IP']
data = json.load(sys.stdin)
corefile = data['data']['Corefile']
block = f'''    hosts {{
       {host_ip} host.docker.internal
       fallthrough
    }}
'''
needle = '    kubernetes cluster.local in-addr.arpa ip6.arpa {'
if needle not in corefile:
    sys.exit('unexpected CoreDNS Corefile layout')
data['data']['Corefile'] = corefile.replace(needle, block + needle, 1)
json.dump(data, sys.stdout)
" | kubectl apply -f -
  kubectl rollout restart deployment/coredns -n kube-system
  kubectl rollout status deployment/coredns -n kube-system --timeout=120s
  echo "CoreDNS now resolves host.docker.internal -> $HOST_IP"
fi
