package server

import (
	"testing"

	"github.com/trains-io/z21.go/protocol"
)

func TestXFirmwareReply(t *testing.T) {
	s := &Server{}
	reply := s.xFirmwareReply()

	fw, err := protocol.ParseXFirmware(reply.Data)
	if err != nil {
		t.Fatalf("ParseXFirmware: %v", err)
	}
	if got := protocol.FormatXFirmwareVersion(fw); got != "1.20" {
		t.Fatalf("FormatXFirmwareVersion() = %q, want 1.20", got)
	}
}

func TestXStatusReplyTrackPowerOn(t *testing.T) {
	s := &Server{trackPower: true}
	reply := s.xStatusReply()

	status, err := protocol.ParseXStatus(reply.Data)
	if err != nil {
		t.Fatalf("ParseXStatus: %v", err)
	}
	if got := protocol.FormatXStatusFlags(status.CentralState); got != "ok" {
		t.Fatalf("CentralState = %q, want ok", got)
	}
}

func TestTrackPowerBCReply(t *testing.T) {
	s := &Server{}

	on, err := protocol.ParseTrackPowerBC(s.trackPowerBCReply(true).Data)
	if err != nil {
		t.Fatalf("ParseTrackPowerBC(on): %v", err)
	}
	if !on {
		t.Fatal("expected track power on")
	}

	off, err := protocol.ParseTrackPowerBC(s.trackPowerBCReply(false).Data)
	if err != nil {
		t.Fatalf("ParseTrackPowerBC(off): %v", err)
	}
	if off {
		t.Fatal("expected track power off")
	}
}
