package server

import (
	"testing"

	"github.com/trains-io/z21.go/protocol"
)

func TestEncodeSystemStateTrackPowerOn(t *testing.T) {
	s := &Server{trackPower: true}
	state, err := protocol.ParseSystemState(s.encodeSystemState())
	if err != nil {
		t.Fatalf("ParseSystemState: %v", err)
	}
	if got := protocol.FormatXStatusFlags(state.CentralState); got != "ok" {
		t.Fatalf("CentralState = %q, want ok", got)
	}
}

func TestEncodeSystemStateTrackPowerOff(t *testing.T) {
	s := &Server{trackPower: false}
	state, err := protocol.ParseSystemState(s.encodeSystemState())
	if err != nil {
		t.Fatalf("ParseSystemState: %v", err)
	}
	if got := protocol.FormatXStatusFlags(state.CentralState); got != "track voltage off" {
		t.Fatalf("CentralState = %q, want track voltage off", got)
	}
}
