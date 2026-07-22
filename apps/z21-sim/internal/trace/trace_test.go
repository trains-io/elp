package trace_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/trains-io/elp/apps/z21-sim/internal/trace"
	"github.com/trains-io/z21.go/protocol"
)

func TestLogLANDumpsDatasetName(t *testing.T) {
	var buf bytes.Buffer
	tr := trace.New(&buf)

	payload, err := protocol.GetHWInfo().Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	tr.LogLAN(trace.DirectionRX, "127.0.0.1:42105", payload)

	out := buf.String()
	if !strings.Contains(out, "LAN << 127.0.0.1:42105") {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, "LAN_GET_HWINFO") {
		t.Fatalf("output = %q", out)
	}
}

func TestLogLANBroadcastNoRecipients(t *testing.T) {
	var buf bytes.Buffer
	tr := trace.New(&buf)

	payload, err := protocol.MarshalAll(protocol.Message{
		Header: protocol.HeaderLANCANDetector,
		Data:   []byte{0x04, 0xdb, 0x00, 0x01},
	})
	if err != nil {
		t.Fatalf("MarshalAll: %v", err)
	}

	tr.LogLANBroadcast(protocol.BroadcastFlagCANDetector, nil, payload)

	out := buf.String()
	if !strings.Contains(out, "LAN >> broadcast CAN detector (0 recipients)") {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, "LAN_CAN_DETECTOR") {
		t.Fatalf("output = %q", out)
	}
}

func TestLogGRPC(t *testing.T) {
	var buf bytes.Buffer
	tr := trace.New(&buf)

	tr.LogGRPC("/trains.io.z21sim.control.v1.SimulatorService/SetTrackPower", nil)

	out := buf.String()
	if !strings.Contains(out, "gRPC << /trains.io.z21sim.control.v1.SimulatorService/SetTrackPower") {
		t.Fatalf("output = %q", out)
	}
}
