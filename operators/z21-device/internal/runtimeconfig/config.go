package runtimeconfig

import (
	"strconv"
	"strings"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// Config holds elp-wide runtime options from the elp-config ConfigMap.
type Config struct {
	GatewayTrace   bool
	SimulatorTrace bool
}

// FromConfigMap parses elp-config data. Missing or invalid keys default to false.
func FromConfigMap(data map[string]string) Config {
	if len(data) == 0 {
		return Config{}
	}
	return Config{
		GatewayTrace:   parseBool(data[z21v1alpha1.ELPConfigKeyGatewayTrace]),
		SimulatorTrace: parseBool(data[z21v1alpha1.ELPConfigKeySimulatorTrace]),
	}
}

func parseBool(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	return err == nil && v
}
