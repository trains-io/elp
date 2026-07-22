package simulation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/trains-io/elp/apps/elpctl/internal/client"
)

const (
	defaultDetectorNetID         uint16 = 0xDB04
	defaultDetectorName                 = "occupancy-a"
	defaultDetectorModuleAddress uint16 = 1
	defaultDetectorPortCount     uint16 = 8
)

// ParseNetID parses a decimal or 0x-prefixed hexadecimal CAN net ID.
func ParseNetID(raw string) (uint16, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("net ID is required")
	}
	base := 10
	if strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X") {
		base = 16
		raw = raw[2:]
	}
	value, err := strconv.ParseUint(raw, base, 16)
	if err != nil {
		return 0, fmt.Errorf("parse net ID %q: %w", raw, err)
	}
	if value == 0 || value > 0xFFFF {
		return 0, fmt.Errorf("net ID %q out of range", raw)
	}
	return uint16(value), nil
}

// DetectorsFromNetIDs builds API detector payloads with default module/port settings.
func DetectorsFromNetIDs(netIDs []uint16) []client.SimulationCANDetector {
	if len(netIDs) == 0 {
		return []client.SimulationCANDetector{{
			NetID:         defaultDetectorNetID,
			Name:          defaultDetectorName,
			ModuleAddress: defaultDetectorModuleAddress,
			PortCount:     defaultDetectorPortCount,
		}}
	}
	out := make([]client.SimulationCANDetector, 0, len(netIDs))
	for i, netID := range netIDs {
		name := defaultDetectorName
		if len(netIDs) > 1 {
			name = fmt.Sprintf("detector-%d", i+1)
		}
		out = append(out, client.SimulationCANDetector{
			NetID:         netID,
			Name:          name,
			ModuleAddress: defaultDetectorModuleAddress,
			PortCount:     defaultDetectorPortCount,
		})
	}
	return out
}
