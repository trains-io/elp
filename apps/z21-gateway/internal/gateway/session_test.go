package gateway

import (
	"context"
	"testing"

	"github.com/trains-io/z21.go/protocol"
)

func TestZ21SessionApplyBroadcastFlagsWithoutClient(t *testing.T) {
	session := newZ21Session(protocol.DefaultBroadcastFlags)
	if err := session.applyBroadcastFlags(context.Background(), 0x102, nil); err != nil {
		t.Fatalf("applyBroadcastFlags() error = %v", err)
	}
	if session.flagsValue() != 0x102 {
		t.Fatalf("flags = %#x", session.flagsValue())
	}
}

func TestZ21SessionSetFlagsDefaults(t *testing.T) {
	session := newZ21Session(0)
	if session.flagsValue() != protocol.DefaultBroadcastFlags {
		t.Fatalf("flags = %#x", session.flagsValue())
	}
}
