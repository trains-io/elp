package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/trains-io/elp/packages/events"
)

func TestHandleControlSetBroadcastFlags(t *testing.T) {
	session := newZ21Session(0x101)
	payload, err := json.Marshal(events.ControlCommand{
		Type:           events.CommandTypeSetBroadcastFlags,
		BroadcastFlags: 0x103,
	})
	if err != nil {
		t.Fatal(err)
	}

	handleControlMessage(context.Background(), session, nil, payload)
	if session.flagsValue() != 0x103 {
		t.Fatalf("flags = %#x", session.flagsValue())
	}
}
