package server

import (
	"testing"
	"time"
)

func TestMakeSimulatorSerialNumber(t *testing.T) {
	serial := makeSimulatorSerialNumber(time.Unix(1_730_123_456, 0))
	if serial/1_000_000 != simSerialPrefix {
		t.Fatalf("serial prefix = %d, want %d", serial/1_000_000, simSerialPrefix)
	}
	if serial%1_000_000 != 123_456 {
		t.Fatalf("serial suffix = %d, want 123456", serial%1_000_000)
	}
}
