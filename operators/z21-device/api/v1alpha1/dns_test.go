package v1alpha1

import "testing"

func TestEnsureClusterDNSFQDN(t *testing.T) {
	if got := EnsureClusterDNSFQDN("nats.default.svc.cluster.local"); got != "nats.default.svc.cluster.local." {
		t.Fatalf("got %q", got)
	}
	if got := EnsureClusterDNSFQDN("nats.default.svc.cluster.local."); got != "nats.default.svc.cluster.local." {
		t.Fatalf("got %q", got)
	}
	if got := EnsureClusterDNSFQDN("192.168.0.1"); got != "192.168.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureNATSURLClusterDNS(t *testing.T) {
	raw := "nats://nats.default.svc.cluster.local:4222"
	want := "nats://nats.default.svc.cluster.local.:4222"
	if got := EnsureNATSURLClusterDNS(raw); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSimulatorServiceFQDNTrailingDot(t *testing.T) {
	if got := SimulatorServiceFQDN("elp", "lab"); got != "z21-sim-lab.elp.svc.cluster.local." {
		t.Fatalf("got %q", got)
	}
}
