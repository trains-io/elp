package runtimeconfig

import (
	"testing"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

func TestFromConfigMapDefaults(t *testing.T) {
	cfg := FromConfigMap(nil)
	if cfg.GatewayTrace || cfg.SimulatorTrace {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestFromConfigMapParsesTraceFlags(t *testing.T) {
	cfg := FromConfigMap(map[string]string{
		z21v1alpha1.ELPConfigKeyGatewayTrace:   "true",
		z21v1alpha1.ELPConfigKeySimulatorTrace: "1",
	})
	if !cfg.GatewayTrace || !cfg.SimulatorTrace {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestFromConfigMapInvalidValues(t *testing.T) {
	cfg := FromConfigMap(map[string]string{
		z21v1alpha1.ELPConfigKeyGatewayTrace: "maybe",
	})
	if cfg.GatewayTrace {
		t.Fatalf("cfg = %#v", cfg)
	}
}
