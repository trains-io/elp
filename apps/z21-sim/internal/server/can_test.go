package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/trains-io/elp/apps/z21-sim/internal/server"
	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

func TestGetCANDeviceDescription(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const netID uint16 = 0xDB04
	setMsg, err := protocol.SetCANDeviceDescription(netID, "Detector 4")
	if err != nil {
		t.Fatalf("SetCANDeviceDescription: %v", err)
	}
	if err := c.Send(ctx, setMsg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	msgs, err := c.Call(ctx, protocol.GetCANDeviceDescription(netID))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var gotName string
	for _, msg := range msgs {
		if msg.Header != protocol.HeaderLANCANDeviceGetDescription {
			continue
		}
		id, name, err := protocol.ParseCANDeviceDescription(msg.Data)
		if err != nil {
			t.Fatalf("ParseCANDeviceDescription: %v", err)
		}
		if id != netID {
			t.Fatalf("netID = %#x, want %#x", id, netID)
		}
		gotName = name
	}
	if gotName != "Detector 4" {
		t.Fatalf("name = %q, want Detector 4", gotName)
	}
}

func TestCANDetectorPoll(t *testing.T) {
	srv, addr := startTestServer(t)

	device, err := server.BuildCANDevice(0xDB04, server.CANDeviceDetector, "Det", 31, 2, 0)
	if err != nil {
		t.Fatalf("BuildCANDevice: %v", err)
	}
	srv.NotifyCANDeviceDetected(device)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, protocol.GetCANDetector(0xDB04)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}

	reports, err := protocol.CANDetectorReportsFromMessages(msgs)
	if err != nil {
		t.Fatalf("CANDetectorReportsFromMessages: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("len(reports) = %d, want 2", len(reports))
	}
	if reports[0].Addr != 31 {
		t.Fatalf("Addr = %d, want 31", reports[0].Addr)
	}
}

func TestNotifyCANDeviceDetectedBroadcast(t *testing.T) {
	srv, addr := startTestServer(t)

	c, err := client.DialLocal(addr, 42130)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, protocol.SetBroadcastFlags(protocol.BroadcastFlagCANDetector)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}
	if _, err := c.Call(ctx, protocol.GetBroadcastFlags()); err != nil {
		t.Fatalf("GetBroadcastFlags: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		device, err := server.BuildCANDevice(0xDB04, server.CANDeviceDetector, "Det", 60, 1, 0)
		if err != nil {
			t.Errorf("BuildCANDevice: %v", err)
			return
		}
		srv.NotifyCANDeviceDetected(device)
	}()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}

	reports, err := protocol.CANDetectorReportsFromMessages(msgs)
	if err != nil {
		t.Fatalf("CANDetectorReportsFromMessages: %v", err)
	}
	if len(reports) != 1 || reports[0].NetID != 0xDB04 {
		t.Fatalf("reports = %+v", reports)
	}
}

func TestCANBoosterTrackPower(t *testing.T) {
	srv, addr := startTestServer(t)

	device, err := server.BuildCANDevice(0xC101, server.CANDeviceBooster, "Booster 1", 0, 0, 1)
	if err != nil {
		t.Fatalf("BuildCANDevice: %v", err)
	}
	srv.NotifyCANDeviceDetected(device)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, protocol.SetCANBoosterTrackPower(0xC101, protocol.CANBoosterTrackPowerDeactivateAll)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}

	states, err := protocol.CANBoosterSystemStatesFromMessages(msgs)
	if err != nil {
		t.Fatalf("CANBoosterSystemStatesFromMessages: %v", err)
	}
	if !protocol.HasCANBoosterState(states[0].State, protocol.CANBoosterStateTrackVoltageOff) {
		t.Fatalf("state = %#x, want TrackVoltageOff", states[0].State)
	}
}
