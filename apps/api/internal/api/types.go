package api

import (
	"fmt"
	"net"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type DeviceList struct {
	Items []Device `json:"items"`
}

type Device struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	Address        string         `json:"address"`
	NATS           NatsConfig     `json:"nats"`
	Gateway        *GatewayConfig `json:"gateway,omitempty"`
	BroadcastFlags uint32         `json:"broadcastFlags"`
	Status         *DeviceStatus  `json:"status,omitempty"`
}

type DeviceCreate struct {
	Name           string         `json:"name"`
	Address        string         `json:"address,omitempty"`
	Backend        *BackendCreate `json:"backend,omitempty"`
	NATS           NatsConfig     `json:"nats"`
	Gateway        *GatewayConfig `json:"gateway,omitempty"`
	BroadcastFlags *uint32        `json:"broadcastFlags,omitempty"`
}

type BackendCreate struct {
	Type      string            `json:"type"`
	Hardware  *HardwareCreate   `json:"hardware,omitempty"`
	Simulator *SimulatorCreate  `json:"simulator,omitempty"`
}

type HardwareCreate struct {
	Host string `json:"host"`
	Port int32  `json:"port,omitempty"`
}

type SimulatorCreate struct {
	Image string `json:"image,omitempty"`
}

type DeviceUpdate struct {
	Address        *string        `json:"address,omitempty"`
	NATS           *NatsConfig    `json:"nats,omitempty"`
	Gateway        *GatewayConfig `json:"gateway,omitempty"`
	BroadcastFlags *uint32        `json:"broadcastFlags,omitempty"`
}

type NatsConfig struct {
	URL           string `json:"url"`
	SubjectPrefix string `json:"subjectPrefix,omitempty"`
}

type GatewayConfig struct {
	Image        string            `json:"image,omitempty"`
	HostNetwork  bool              `json:"hostNetwork,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

type DeviceStatus struct {
	Phase                     string     `json:"phase,omitempty"`
	GatewayDeployment         string     `json:"gatewayDeployment,omitempty"`
	GatewayReady              bool       `json:"gatewayReady,omitempty"`
	DeviceReachable           bool       `json:"deviceReachable,omitempty"`
	Degraded                  bool       `json:"degraded,omitempty"`
	HealthCheckFailures       int32      `json:"healthCheckFailures,omitempty"`
	LastSuccessfulHealthCheck *time.Time `json:"lastSuccessfulHealthCheck,omitempty"`
	LastSeen                  *time.Time `json:"lastSeen,omitempty"`
	SerialNumber              string     `json:"serialNumber,omitempty"`
	FirmwareVersion           string     `json:"firmwareVersion,omitempty"`
	HardwareType              string     `json:"hardwareType,omitempty"`
	XBusVersion               string     `json:"xbusVersion,omitempty"`
	XBusFirmwareVersion       string     `json:"xbusFirmwareVersion,omitempty"`
	LockCode                  *uint8     `json:"lockCode,omitempty"`
}

func deviceFromCR(cr *z21v1alpha1.Z21Device) Device {
	return DeviceFromCR(cr)
}

// DeviceFromCR maps a Z21Device CR to the API Device type.
func DeviceFromCR(cr *z21v1alpha1.Z21Device) Device {
	addr, _ := cr.Z21Address()
	out := Device{
		Name:           cr.Name,
		Namespace:      cr.Namespace,
		Address:        addr,
		BroadcastFlags: cr.BroadcastFlagsValue(),
		NATS: NatsConfig{
			URL:           cr.Spec.NATS.URL,
			SubjectPrefix: cr.Spec.NATS.SubjectPrefix,
		},
		Status: statusFromCR(cr),
	}

	if cr.Spec.Gateway.Image != "" || cr.Spec.Gateway.HostNetwork || len(cr.Spec.Gateway.NodeSelector) > 0 {
		out.Gateway = &GatewayConfig{
			Image:        cr.Spec.Gateway.Image,
			HostNetwork:  cr.Spec.Gateway.HostNetwork,
			NodeSelector: cr.Spec.Gateway.NodeSelector,
		}
	}
	return out
}

func statusFromCR(cr *z21v1alpha1.Z21Device) *DeviceStatus {
	if cr.Status.Phase == "" && cr.Status.GatewayDeployment == "" &&
		cr.Status.LastSeen == nil && cr.Status.SerialNumber == "" && cr.Status.FirmwareVersion == "" &&
		cr.Status.HardwareType == "" && cr.Status.XBusVersion == "" && cr.Status.XBusFirmwareVersion == "" &&
		cr.Status.LockCode == nil && len(cr.Status.Conditions) == 0 {
		return nil
	}

	status := &DeviceStatus{
		Phase:               string(cr.Status.Phase),
		GatewayDeployment:   cr.Status.GatewayDeployment,
		GatewayReady:        conditionIsTrue(cr.Status.Conditions, z21v1alpha1.ConditionGatewayReady),
		DeviceReachable:     conditionIsTrue(cr.Status.Conditions, z21v1alpha1.ConditionDeviceReachable),
		Degraded:            conditionIsTrue(cr.Status.Conditions, z21v1alpha1.ConditionDegraded),
		HealthCheckFailures: cr.Status.HealthCheckFailures,
		SerialNumber:        cr.Status.SerialNumber,
		FirmwareVersion:     cr.Status.FirmwareVersion,
		HardwareType:        cr.Status.HardwareType,
		XBusVersion:         cr.Status.XBusVersion,
		XBusFirmwareVersion: cr.Status.XBusFirmwareVersion,
		LockCode:            cr.Status.LockCode,
	}
	if cr.Status.LastSeen != nil {
		t := cr.Status.LastSeen.Time
		status.LastSeen = &t
	}
	if cr.Status.LastSuccessfulHealthCheck != nil {
		t := cr.Status.LastSuccessfulHealthCheck.Time
		status.LastSuccessfulHealthCheck = &t
	}
	return status
}

func conditionIsTrue(conditions []metav1.Condition, condType string) bool {
	for _, cond := range conditions {
		if cond.Type == condType && cond.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func specFromCreate(req DeviceCreate) (z21v1alpha1.Z21DeviceSpec, error) {
	backend, err := backendFromCreate(req)
	if err != nil {
		return z21v1alpha1.Z21DeviceSpec{}, err
	}

	spec := z21v1alpha1.Z21DeviceSpec{
		Backend: backend,
		NATS: z21v1alpha1.NatsSpec{
			URL:           req.NATS.URL,
			SubjectPrefix: req.NATS.SubjectPrefix,
		},
	}
	if req.Gateway != nil {
		spec.Gateway = z21v1alpha1.GatewaySpec{
			Image:        req.Gateway.Image,
			HostNetwork:  req.Gateway.HostNetwork,
			NodeSelector: req.Gateway.NodeSelector,
		}
	}
	if req.BroadcastFlags != nil {
		spec.BroadcastFlags = req.BroadcastFlags
	} else {
		flags := z21v1alpha1.DefaultBroadcastFlags
		spec.BroadcastFlags = &flags
	}
	return spec, nil
}

func validateDeviceCreate(req DeviceCreate) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.NATS.URL == "" {
		return fmt.Errorf("nats.url is required")
	}
	if req.Backend != nil && req.Address != "" {
		return fmt.Errorf("use either address or backend, not both")
	}
	if req.Backend == nil && req.Address == "" {
		return fmt.Errorf("address or backend is required")
	}
	if req.Backend != nil {
		if req.Backend.Type == "" {
			return fmt.Errorf("backend.type is required")
		}
		switch z21v1alpha1.BackendType(req.Backend.Type) {
		case z21v1alpha1.BackendHardware:
			if req.Backend.Simulator != nil {
				return fmt.Errorf("backend.simulator must not be set for hardware backend")
			}
		case z21v1alpha1.BackendSimulator:
			if req.Backend.Hardware != nil {
				return fmt.Errorf("backend.hardware must not be set for simulator backend")
			}
		default:
			return fmt.Errorf("unsupported backend type %q", req.Backend.Type)
		}
	}
	return nil
}

func backendFromCreate(req DeviceCreate) (z21v1alpha1.BackendSpec, error) {
	if req.Backend != nil {
		switch z21v1alpha1.BackendType(req.Backend.Type) {
		case z21v1alpha1.BackendHardware:
			hw, err := hardwareFromCreate(req.Backend.Hardware, "")
			if err != nil {
				return z21v1alpha1.BackendSpec{}, err
			}
			return z21v1alpha1.BackendSpec{
				Type:     z21v1alpha1.BackendHardware,
				Hardware: &hw,
			}, nil
		case z21v1alpha1.BackendSimulator:
			var sim *z21v1alpha1.SimulatorBackend
			if req.Backend.Simulator != nil {
				sim = &z21v1alpha1.SimulatorBackend{
					Image: req.Backend.Simulator.Image,
				}
			}
			return z21v1alpha1.BackendSpec{
				Type:      z21v1alpha1.BackendSimulator,
				Simulator: sim,
			}, nil
		default:
			return z21v1alpha1.BackendSpec{}, fmt.Errorf("unsupported backend type %q", req.Backend.Type)
		}
	}

	hw, err := parseHardwareAddress(req.Address)
	if err != nil {
		return z21v1alpha1.BackendSpec{}, err
	}
	return z21v1alpha1.BackendSpec{
		Type:     z21v1alpha1.BackendHardware,
		Hardware: &hw,
	}, nil
}

func hardwareFromCreate(hw *HardwareCreate, address string) (z21v1alpha1.HardwareBackend, error) {
	if address != "" {
		if hw != nil && (hw.Host != "" || hw.Port != 0) {
			return z21v1alpha1.HardwareBackend{}, fmt.Errorf("use either address or backend.hardware, not both")
		}
		return parseHardwareAddress(address)
	}
	if hw == nil || hw.Host == "" {
		return z21v1alpha1.HardwareBackend{}, fmt.Errorf("backend.hardware.host is required for hardware backend")
	}
	out := z21v1alpha1.HardwareBackend{
		Host: hw.Host,
		Port: hw.Port,
	}
	if out.Port != 0 && (out.Port < 1 || out.Port > 65535) {
		return z21v1alpha1.HardwareBackend{}, fmt.Errorf("invalid backend.hardware.port %d", out.Port)
	}
	return out, nil
}

func applyUpdate(spec *z21v1alpha1.Z21DeviceSpec, req DeviceUpdate) error {
	if req.Address != nil {
		hw, err := parseHardwareAddress(*req.Address)
		if err != nil {
			return err
		}
		spec.Backend.Type = z21v1alpha1.BackendHardware
		spec.Backend.Hardware = &hw
		spec.Backend.Simulator = nil
	}
	if req.NATS != nil {
		spec.NATS = z21v1alpha1.NatsSpec{
			URL:           req.NATS.URL,
			SubjectPrefix: req.NATS.SubjectPrefix,
		}
	}
	if req.Gateway != nil {
		spec.Gateway = z21v1alpha1.GatewaySpec{
			Image:        req.Gateway.Image,
			HostNetwork:  req.Gateway.HostNetwork,
			NodeSelector: req.Gateway.NodeSelector,
		}
	}
	if req.BroadcastFlags != nil {
		spec.BroadcastFlags = req.BroadcastFlags
	}
	return nil
}

func parseHardwareAddress(addr string) (z21v1alpha1.HardwareBackend, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return z21v1alpha1.HardwareBackend{Host: addr}, nil
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil || port < 1 || port > 65535 {
		return z21v1alpha1.HardwareBackend{}, fmt.Errorf("invalid address port %q", portStr)
	}
	return z21v1alpha1.HardwareBackend{
		Host: host,
		Port: int32(port),
	}, nil
}
