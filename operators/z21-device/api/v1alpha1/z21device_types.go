package v1alpha1

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DisplayNameAnnotation = "z21.trains.io/display-name"
	DeviceFinalizerName   = "z21.trains.io/finalizer"

	DefaultZ21Port = int32(21105)
)

var (
	DefaultHealthCheckInterval         = metav1.Duration{Duration: 2 * time.Second}
	DefaultHealthCheckTimeout          = metav1.Duration{Duration: 500 * time.Millisecond}
	DefaultHealthCheckFailureThreshold = int32(3)
	DefaultHealthCheckSuccessThreshold = int32(1)
)

// DevicePhase reports gateway lifecycle state reported by the controller.
type DevicePhase string

const (
	PhasePending  DevicePhase = "Pending"
	PhaseStarting DevicePhase = "Starting"
	PhaseRunning  DevicePhase = "Running"
	PhaseFailed   DevicePhase = "Failed"
	PhaseStopping DevicePhase = "Stopping"
)

// BackendType selects how the Z21 command station is reached.
type BackendType string

const (
	BackendHardware  BackendType = "hardware"
	BackendSimulator BackendType = "simulator"
)

// Condition types reported on Z21Device status.
const (
	ConditionGatewayReady    = "GatewayReady"
	ConditionDeviceReachable = "DeviceReachable"
	ConditionDegraded        = "Degraded"
)

// Z21DeviceSpec defines the desired state of a Z21 command station reachable from the cluster.
type Z21DeviceSpec struct {
	// Backend selects hardware on the LAN or an in-cluster z21 simulator.
	// +kubebuilder:validation:Required
	Backend BackendSpec `json:"backend"`

	// NATS configures how the gateway publishes device events.
	// +kubebuilder:validation:Required
	NATS NatsSpec `json:"nats"`

	// HealthCheck configures gateway device reachability probing.
	// +optional
	HealthCheck HealthCheckSpec `json:"healthCheck,omitempty"`

	// Gateway configures the gateway Deployment reconciled for this device.
	// +optional
	Gateway GatewaySpec `json:"gateway,omitempty"`

	// BroadcastFlags is the LAN_SET_BROADCASTFLAGS bitmask (spec §2.16).
	// When unset the gateway uses 0x00000101 (XpressNet + system state).
	// +optional
	BroadcastFlags *uint32 `json:"broadcastFlags,omitempty"`

	// CANAddressPoolRef names the CANAddressPool used for module address allocation.
	// When unset, the controller uses the pool named "default" in the elp namespace.
	// +optional
	CANAddressPoolRef *corev1.LocalObjectReference `json:"canAddressPoolRef,omitempty"`
}

// BackendSpec describes the Z21 command station endpoint.
type BackendSpec struct {
	// Type is hardware for a LAN-connected command station or simulator for z21-sim.
	// +kubebuilder:validation:Enum=hardware;simulator
	// +kubebuilder:validation:Required
	Type BackendType `json:"type"`

	// Hardware is required when type is hardware.
	// +optional
	Hardware *HardwareBackend `json:"hardware,omitempty"`

	// Simulator is optional when type is simulator.
	// +optional
	Simulator *SimulatorBackend `json:"simulator,omitempty"`
}

// HardwareBackend is a UDP endpoint on the LAN.
type HardwareBackend struct {
	// Host is the command station IP address or resolvable hostname.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// Port is the Z21 UDP port. Defaults to 21105.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

// SimulatorBackend runs z21-sim in the cluster for this device.
type SimulatorBackend struct {
	// Image is the simulator container image.
	// +optional
	Image string `json:"image,omitempty"`
}

// HealthCheckSpec configures gateway reachability probing against the Z21 endpoint.
type HealthCheckSpec struct {
	// Interval between probe attempts.
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout for a single probe attempt.
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// FailureThreshold is the number of consecutive failures before DeviceReachable becomes False.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes before DeviceReachable becomes True.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
}

// NatsSpec holds NATS connection settings for the gateway.
type NatsSpec struct {
	// URL is the NATS server URL, for example "nats://nats.nats.svc:4222".
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// SubjectPrefix is prepended to published subjects. When empty the controller
	// defaults to "z21.<namespace>.<name>".
	// +optional
	SubjectPrefix string `json:"subjectPrefix,omitempty"`
}

// GatewaySpec configures the per-device gateway Deployment.
type GatewaySpec struct {
	// Image is the gateway container image. When empty a controller default is used.
	// +optional
	Image string `json:"image,omitempty"`

	// HostNetwork runs the gateway pod with host networking for LAN reachability.
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// NodeSelector schedules the gateway onto nodes that can reach the device.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// HostAliases adds static /etc/hosts entries on the gateway pod.
	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`
}

// Z21DeviceStatus defines the observed state of Z21Device.
type Z21DeviceStatus struct {
	// Phase is the high-level gateway lifecycle state.
	// +optional
	Phase DevicePhase `json:"phase,omitempty"`

	// Conditions report gateway readiness and device connectivity.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSuccessfulHealthCheck is updated by the gateway after a successful probe.
	// +optional
	LastSuccessfulHealthCheck *metav1.Time `json:"lastSuccessfulHealthCheck,omitempty"`

	// HealthCheckFailures is the current consecutive failed probe count.
	// +optional
	HealthCheckFailures int32 `json:"healthCheckFailures,omitempty"`

	// GatewayDeployment is the name of the reconciled gateway Deployment.
	// +optional
	GatewayDeployment string `json:"gatewayDeployment,omitempty"`

	// SimulatorDeployment is the name of the reconciled simulator Deployment.
	// +optional
	SimulatorDeployment string `json:"simulatorDeployment,omitempty"`

	// SimulatorService is the ClusterIP Service fronting the simulator.
	// +optional
	SimulatorService string `json:"simulatorService,omitempty"`

	// LastSeen is the last time the gateway reported a successful Z21 interaction.
	// +optional
	LastSeen *metav1.Time `json:"lastSeen,omitempty"`

	// SerialNumber is the Z21 hardware serial reported by the gateway.
	// +optional
	SerialNumber string `json:"serialNumber,omitempty"`

	// FirmwareVersion is the Z21 firmware version string reported by the gateway.
	// +optional
	FirmwareVersion string `json:"firmwareVersion,omitempty"`

	// HardwareType is the Z21 hardware type from LAN_GET_HWINFO (hex string).
	// +optional
	HardwareType string `json:"hardwareType,omitempty"`

	// XBusVersion is the X-Bus protocol version from LAN_X_GET_VERSION.
	// +optional
	XBusVersion string `json:"xbusVersion,omitempty"`

	// XBusFirmwareVersion is the X-Bus firmware version from LAN_X_GET_FIRMWARE_VERSION.
	// +optional
	XBusFirmwareVersion string `json:"xbusFirmwareVersion,omitempty"`

	// LockCode is the LAN_GET_CODE feature lock byte (0 = no lock, 1 = z21 start locked, 2 = unlocked).
	// +optional
	LockCode *uint8 `json:"lockCode,omitempty"`

	// ObservedGeneration reflects the generation of the spec the controller reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=z21
// +kubebuilder:printcolumn:name="Backend",type=string,JSONPath=`.spec.backend.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.status.conditions[?(@.type=="GatewayReady")].status`
// +kubebuilder:printcolumn:name="Reachable",type=string,JSONPath=`.status.conditions[?(@.type=="DeviceReachable")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="!has(self.spec) || self.spec.backend.type != 'hardware' || has(self.spec.backend.hardware)",message="backend.hardware is required when backend.type is hardware"
// +kubebuilder:validation:XValidation:rule="!has(self.spec) || self.spec.backend.type != 'simulator' || !has(self.spec.backend.hardware)",message="backend.hardware must not be set when backend.type is simulator"

// Z21Device represents a physical or virtual Z21 command station managed in-cluster.
type Z21Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Z21DeviceSpec   `json:"spec,omitempty"`
	Status Z21DeviceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Z21DeviceList contains a list of Z21Device.
type Z21DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Z21Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Z21Device{}, &Z21DeviceList{})
}

// SubjectPrefix returns the NATS subject prefix for this device.
func (d *Z21Device) SubjectPrefix() string {
	if p := d.Spec.NATS.SubjectPrefix; p != "" {
		return p
	}
	return "z21." + d.Namespace + "." + d.Name
}

const (
	broadcastFlagXpressNet     uint32 = 0x00000001
	broadcastFlagSystemState   uint32 = 0x00000100
	broadcastFlagCANDetector   uint32 = 0x00080000
)

// DefaultBroadcastFlags is used when spec.broadcastFlags is unset.
const DefaultBroadcastFlags uint32 = broadcastFlagXpressNet | broadcastFlagSystemState | broadcastFlagCANDetector

// DefaultSimulatorImage is the in-cluster z21 simulator image.
const DefaultSimulatorImage = "ghcr.io/trains-io/z21-sim:latest"

// DefaultSimulatorGRPCPort is the z21-sim gRPC control plane port.
const DefaultSimulatorGRPCPort = 50051

// DefaultWorkloadNamespace is where gateway and simulator workloads are reconciled.
const DefaultWorkloadNamespace = "elp"

// DefaultNATSURL is the in-cluster NATS URL for gateways and the API.
// The trailing dot on the hostname avoids bogus lookups when host DNS search
// domains leak into pod resolv.conf (common on WSL/Docker Desktop).
const DefaultNATSURL = "nats://nats.default.svc.cluster.local.:4222"

// DefaultCANAddressPoolName is used when spec.canAddressPoolRef is unset.
const DefaultCANAddressPoolName = "default"

// DefaultCANAddressPoolNamespace is where the bundled default pool is installed.
const DefaultCANAddressPoolNamespace = DefaultWorkloadNamespace

// DefaultCANAddressPoolStart is the first auto-allocatable address in the default pool.
const DefaultCANAddressPoolStart uint16 = 10

// DefaultCANAddressPoolEnd is the last auto-allocatable address in the default pool.
const DefaultCANAddressPoolEnd uint16 = 200

// DefaultCANAddressPoolReserved are never auto-allocated in the default pool.
var DefaultCANAddressPoolReserved = []uint16{1, 2, 3}

// DefaultCANAddressPoolSpec returns the bundled default pool configuration.
func DefaultCANAddressPoolSpec() CANAddressPoolSpec {
	return CANAddressPoolSpec{
		Start:    DefaultCANAddressPoolStart,
		End:      DefaultCANAddressPoolEnd,
		Reserved: append([]uint16(nil), DefaultCANAddressPoolReserved...),
	}
}

// BroadcastFlagsValue returns the effective broadcast flags for this device.
func (d *Z21Device) BroadcastFlagsValue() uint32 {
	if d.Spec.BroadcastFlags != nil {
		return *d.Spec.BroadcastFlags
	}
	return DefaultBroadcastFlags
}

// HealthCheckSpecOrDefaults returns spec.healthCheck with defaults applied.
func (d *Z21Device) HealthCheckSpecOrDefaults() HealthCheckSpec {
	spec := d.Spec.HealthCheck
	if spec.Interval.Duration == 0 {
		spec.Interval = DefaultHealthCheckInterval
	}
	if spec.Timeout.Duration == 0 {
		spec.Timeout = DefaultHealthCheckTimeout
	}
	if spec.FailureThreshold == 0 {
		spec.FailureThreshold = DefaultHealthCheckFailureThreshold
	}
	if spec.SuccessThreshold == 0 {
		spec.SuccessThreshold = DefaultHealthCheckSuccessThreshold
	}
	return spec
}

// Z21Address returns the UDP endpoint the gateway should dial.
func (d *Z21Device) Z21Address() (string, error) {
	switch d.Spec.Backend.Type {
	case BackendHardware:
		if d.Spec.Backend.Hardware == nil {
			return "", fmt.Errorf("backend.hardware is required for hardware backend")
		}
		port := d.Spec.Backend.Hardware.Port
		if port == 0 {
			port = DefaultZ21Port
		}
		return net.JoinHostPort(d.Spec.Backend.Hardware.Host, fmt.Sprintf("%d", port)), nil
	case BackendSimulator:
		return net.JoinHostPort(SimulatorServiceFQDN(DefaultWorkloadNamespace, d.Name), fmt.Sprintf("%d", DefaultZ21Port)), nil
	default:
		return "", fmt.Errorf("unsupported backend type %q", d.Spec.Backend.Type)
	}
}

// SimulatorServiceName returns the in-cluster Service DNS label for the simulator.
func SimulatorServiceName(deviceName string) string {
	return fmt.Sprintf("z21-sim-%s", deviceName)
}

// SimulatorServiceFQDN returns the cluster DNS name for the simulator Service.
func SimulatorServiceFQDN(namespace, deviceName string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local.", SimulatorServiceName(deviceName), namespace)
}

// EnsureClusterDNSFQDN appends a trailing dot to *.cluster.local hostnames so
// resolver search paths cannot rewrite in-cluster service names.
func EnsureClusterDNSFQDN(host string) string {
	host = strings.TrimSuffix(host, ".")
	if strings.HasSuffix(host, ".cluster.local") {
		return host + "."
	}
	return host
}

// EnsureNATSURLClusterDNS normalizes in-cluster NATS URLs for reliable DNS.
func EnsureNATSURLClusterDNS(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return raw
	}
	host := EnsureClusterDNSFQDN(u.Hostname())
	if host == u.Hostname() {
		return raw
	}
	port := u.Port()
	if port == "" {
		u.Host = host
	} else {
		u.Host = net.JoinHostPort(host, port)
	}
	return u.String()
}

// SimulatorImage returns the configured simulator image.
func (d *Z21Device) SimulatorImage() string {
	if d.Spec.Backend.Simulator != nil && d.Spec.Backend.Simulator.Image != "" {
		return d.Spec.Backend.Simulator.Image
	}
	return DefaultSimulatorImage
}

// SimulatorGRPCAddress returns the in-cluster gRPC endpoint for z21-sim control.
func SimulatorGRPCAddress(workloadNamespace, deviceName string) string {
	return net.JoinHostPort(
		SimulatorServiceFQDN(workloadNamespace, deviceName),
		fmt.Sprintf("%d", DefaultSimulatorGRPCPort),
	)
}

// CANAddressPoolName returns the effective CANAddressPool name for this device.
func (d *Z21Device) CANAddressPoolName() string {
	if d.Spec.CANAddressPoolRef != nil && d.Spec.CANAddressPoolRef.Name != "" {
		return d.Spec.CANAddressPoolRef.Name
	}
	return DefaultCANAddressPoolName
}

// CANAddressPoolNamespace returns the namespace of the effective CANAddressPool.
func (d *Z21Device) CANAddressPoolNamespace() string {
	if d.Spec.CANAddressPoolRef != nil && d.Spec.CANAddressPoolRef.Name != "" {
		return d.Namespace
	}
	return DefaultCANAddressPoolNamespace
}

// DisplayName returns the human-facing device name from annotations.
func (d *Z21Device) DisplayName() string {
	if name := d.Annotations[DisplayNameAnnotation]; name != "" {
		return name
	}
	return d.Name
}
