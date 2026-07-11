package api

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestDeviceFromCR(t *testing.T) {
	lastSeen := metav1.NewTime(time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC))
	cr := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "basement", Namespace: "trains"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type: z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{
					Host: "192.168.0.42",
				},
			},
			NATS:    z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
			Gateway: z21v1alpha1.GatewaySpec{HostNetwork: true},
		},
		Status: z21v1alpha1.Z21DeviceStatus{
			Phase:    z21v1alpha1.PhaseRunning,
			LastSeen: &lastSeen,
			Conditions: []metav1.Condition{{
				Type:   z21v1alpha1.ConditionDeviceReachable,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	got := deviceFromCR(cr)
	if got.Name != "basement" || got.Address != "192.168.0.42:21105" {
		t.Fatalf("unexpected device: %#v", got)
	}
	if got.Status == nil || got.Status.Phase != "Running" || !got.Status.DeviceReachable {
		t.Fatalf("unexpected status: %#v", got.Status)
	}
	if got.BroadcastFlags != z21v1alpha1.DefaultBroadcastFlags {
		t.Fatalf("broadcastFlags = %#x", got.BroadcastFlags)
	}
}

func TestSpecFromCreateDefaultsBroadcastFlags(t *testing.T) {
	spec, err := specFromCreate(DeviceCreate{
		Name:    "basement",
		Address: "192.168.0.42:21105",
		NATS:    NatsConfig{URL: "nats://nats:4222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.BroadcastFlags == nil {
		t.Fatal("expected broadcastFlags to be set")
	}
	if *spec.BroadcastFlags != z21v1alpha1.DefaultBroadcastFlags {
		t.Fatalf("broadcastFlags = %#x, want %#x", *spec.BroadcastFlags, z21v1alpha1.DefaultBroadcastFlags)
	}
	if spec.Backend.Type != z21v1alpha1.BackendHardware || spec.Backend.Hardware.Host != "192.168.0.42" {
		t.Fatalf("backend = %#v", spec.Backend)
	}
}

func TestSpecFromCreateSimulator(t *testing.T) {
	spec, err := specFromCreate(DeviceCreate{
		Name: "sim-bench",
		Backend: &BackendCreate{
			Type: "simulator",
			Simulator: &SimulatorCreate{
				Image: "ghcr.io/trains-io/z21-sim:latest",
			},
		},
		NATS: NatsConfig{URL: "nats://nats:4222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Backend.Type != z21v1alpha1.BackendSimulator {
		t.Fatalf("backend type = %q", spec.Backend.Type)
	}
	if spec.Backend.Hardware != nil {
		t.Fatal("expected no hardware backend")
	}
	if spec.Backend.Simulator == nil || spec.Backend.Simulator.Image != "ghcr.io/trains-io/z21-sim:latest" {
		t.Fatalf("simulator = %#v", spec.Backend.Simulator)
	}
}

func TestValidateDeviceCreate(t *testing.T) {
	err := validateDeviceCreate(DeviceCreate{
		Name: "sim",
		Backend: &BackendCreate{
			Type: "simulator",
			Hardware: &HardwareCreate{
				Host: "192.168.0.42",
			},
		},
		NATS: NatsConfig{URL: "nats://nats:4222"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDeviceFromCRStatusFields(t *testing.T) {
	lockCode := uint8(0)
	cr := &z21v1alpha1.Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "default"},
		Spec: z21v1alpha1.Z21DeviceSpec{
			Backend: z21v1alpha1.BackendSpec{
				Type: z21v1alpha1.BackendHardware,
				Hardware: &z21v1alpha1.HardwareBackend{
					Host: "host.docker.internal",
				},
			},
			NATS: z21v1alpha1.NatsSpec{URL: "nats://nats:4222"},
		},
		Status: z21v1alpha1.Z21DeviceStatus{
			Phase:               z21v1alpha1.PhaseRunning,
			FirmwareVersion:     "1.43",
			HardwareType:        "0x00000201",
			XBusVersion:         "4.0",
			XBusFirmwareVersion: "1.43",
			LockCode:            &lockCode,
			Conditions: []metav1.Condition{{
				Type:   z21v1alpha1.ConditionGatewayReady,
				Status: metav1.ConditionTrue,
			}},
		},
	}

	got := deviceFromCR(cr)
	if got.Status == nil {
		t.Fatal("expected status")
	}
	if !got.Status.GatewayReady {
		t.Fatal("expected gatewayReady")
	}
	if got.Status.HardwareType != "0x00000201" {
		t.Fatalf("hardwareType = %q", got.Status.HardwareType)
	}
	if got.Status.XBusVersion != "4.0" {
		t.Fatalf("xbusVersion = %q", got.Status.XBusVersion)
	}
	if got.Status.XBusFirmwareVersion != "1.43" {
		t.Fatalf("xbusFirmwareVersion = %q", got.Status.XBusFirmwareVersion)
	}
	if got.Status.LockCode == nil || *got.Status.LockCode != 0 {
		t.Fatalf("lockCode = %v", got.Status.LockCode)
	}
}

func TestParseHardwareAddress(t *testing.T) {
	hw, err := parseHardwareAddress("192.168.0.42:21107")
	if err != nil {
		t.Fatal(err)
	}
	if hw.Host != "192.168.0.42" || hw.Port != 21107 {
		t.Fatalf("parseHardwareAddress() = %#v", hw)
	}

	hw, err = parseHardwareAddress("host.docker.internal")
	if err != nil {
		t.Fatal(err)
	}
	if hw.Host != "host.docker.internal" || hw.Port != 0 {
		t.Fatalf("parseHardwareAddress() = %#v", hw)
	}
}

func TestApplyUpdateAddress(t *testing.T) {
	spec := z21v1alpha1.Z21DeviceSpec{
		Backend: z21v1alpha1.BackendSpec{
			Type: z21v1alpha1.BackendSimulator,
		},
	}
	addr := "10.0.0.5:21105"
	if err := applyUpdate(&spec, DeviceUpdate{Address: &addr}); err != nil {
		t.Fatal(err)
	}
	if spec.Backend.Type != z21v1alpha1.BackendHardware || spec.Backend.Hardware == nil {
		t.Fatalf("backend = %#v", spec.Backend)
	}
	if spec.Backend.Hardware.Host != "10.0.0.5" || spec.Backend.Hardware.Port != 21105 {
		t.Fatalf("hardware = %#v", spec.Backend.Hardware)
	}
	if spec.Backend.Simulator != nil {
		t.Fatal("expected simulator backend to be cleared")
	}
}
