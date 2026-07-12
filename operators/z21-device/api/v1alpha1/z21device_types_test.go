package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultBroadcastFlags(t *testing.T) {
	if DefaultBroadcastFlags != broadcastFlagXpressNet|broadcastFlagSystemState {
		t.Fatalf("DefaultBroadcastFlags = %#x", DefaultBroadcastFlags)
	}
	if DefaultBroadcastFlags != 0x00000101 {
		t.Fatalf("DefaultBroadcastFlags = %#x, want 0x101", DefaultBroadcastFlags)
	}
}

func TestBroadcastFlagsValueUsesDefault(t *testing.T) {
	device := &Z21Device{}
	if device.BroadcastFlagsValue() != DefaultBroadcastFlags {
		t.Fatalf("BroadcastFlagsValue() = %#x", device.BroadcastFlagsValue())
	}
}

func TestCANAddressPoolNameUsesDefault(t *testing.T) {
	device := &Z21Device{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}}
	if device.CANAddressPoolName() != DefaultCANAddressPoolName {
		t.Fatalf("CANAddressPoolName() = %q", device.CANAddressPoolName())
	}
	if device.CANAddressPoolNamespace() != DefaultCANAddressPoolNamespace {
		t.Fatalf("CANAddressPoolNamespace() = %q", device.CANAddressPoolNamespace())
	}

	custom := &Z21Device{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: Z21DeviceSpec{
			CANAddressPoolRef: &corev1.LocalObjectReference{Name: "layout-a"},
		},
	}
	if custom.CANAddressPoolName() != "layout-a" {
		t.Fatalf("CANAddressPoolName() = %q", custom.CANAddressPoolName())
	}
	if custom.CANAddressPoolNamespace() != "default" {
		t.Fatalf("CANAddressPoolNamespace() = %q", custom.CANAddressPoolNamespace())
	}
}

func TestDefaultCANAddressPoolSpec(t *testing.T) {
	spec := DefaultCANAddressPoolSpec()
	if spec.Start != DefaultCANAddressPoolStart || spec.End != DefaultCANAddressPoolEnd {
		t.Fatalf("spec range = %d-%d", spec.Start, spec.End)
	}
	if len(spec.Reserved) != len(DefaultCANAddressPoolReserved) {
		t.Fatalf("reserved = %v", spec.Reserved)
	}
}

func TestZ21AddressHardware(t *testing.T) {
	device := &Z21Device{
		Spec: Z21DeviceSpec{
			Backend: BackendSpec{
				Type: BackendHardware,
				Hardware: &HardwareBackend{
					Host: "192.168.0.42",
					Port: 21107,
				},
			},
		},
	}
	addr, err := device.Z21Address()
	if err != nil {
		t.Fatal(err)
	}
	if addr != "192.168.0.42:21107" {
		t.Fatalf("Z21Address = %q", addr)
	}
}

func TestZ21AddressSimulator(t *testing.T) {
	device := &Z21Device{
		ObjectMeta: metav1.ObjectMeta{Name: "lab", Namespace: "trains"},
		Spec: Z21DeviceSpec{
			Backend: BackendSpec{Type: BackendSimulator},
		},
	}
	addr, err := device.Z21Address()
	if err != nil {
		t.Fatal(err)
	}
	if addr != "z21-sim-lab.elp.svc.cluster.local.:21105" {
		t.Fatalf("Z21Address = %q", addr)
	}
}

func TestHealthCheckSpecOrDefaults(t *testing.T) {
	device := &Z21Device{}
	spec := device.HealthCheckSpecOrDefaults()
	if spec.Interval != DefaultHealthCheckInterval {
		t.Fatalf("interval = %v", spec.Interval)
	}
	if spec.Timeout != DefaultHealthCheckTimeout {
		t.Fatalf("timeout = %v", spec.Timeout)
	}
	if spec.FailureThreshold != DefaultHealthCheckFailureThreshold {
		t.Fatalf("failureThreshold = %d", spec.FailureThreshold)
	}
}
