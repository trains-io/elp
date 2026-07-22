package v1alpha1

// ELPConfigMapName is the cluster-wide runtime defaults ConfigMap in the workload namespace.
const ELPConfigMapName = "elp-config"

const (
	// ELPConfigKeyGatewayTrace enables Z21 LAN message tracing in gateway pods.
	ELPConfigKeyGatewayTrace = "gateway.trace"
	// ELPConfigKeySimulatorTrace enables LAN and gRPC tracing in z21-sim pods.
	ELPConfigKeySimulatorTrace = "simulator.trace"
)
