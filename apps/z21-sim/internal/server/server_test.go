package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/trains-io/elp/apps/z21-sim/internal/server"
	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

func startTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	srv, err := server.New("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("server did not stop cleanly")
		}
		_ = srv.Close()
	})

	return srv, srv.Addr().String()
}

func TestGetHWInfo(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetHWInfo())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	hw, err := protocol.HWInfoFromMessages(msgs)
	if err != nil {
		t.Fatalf("HWInfoFromMessages: %v", err)
	}
	if hw.HwType != protocol.HwTypeZ21New {
		t.Fatalf("HwType = %#x, want %#x", hw.HwType, protocol.HwTypeZ21New)
	}
}

func TestGetSerialNumber(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetSerialNumber())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	serial, ok := protocol.SerialFromMessages(msgs)
	if !ok {
		t.Fatalf("SerialFromMessages() ok = false, msgs = %#v", msgs)
	}
	if len(serial) < 4 || serial[:3] != "999" {
		t.Fatalf("serial = %q, want 999-prefix simulator serial", serial)
	}
}

func TestGetCode(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetCode())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	code, err := protocol.CodeFromMessages(msgs)
	if err != nil {
		t.Fatalf("CodeFromMessages: %v", err)
	}
	if code != protocol.CodeNoLock {
		t.Fatalf("code = %#x, want %#x", code, protocol.CodeNoLock)
	}
}

func TestGetBroadcastFlags(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const flags uint32 = 0x103
	if err := c.Send(ctx, protocol.SetBroadcastFlags(flags)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}

	msgs, err := c.Call(ctx, protocol.GetBroadcastFlags())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	got, err := protocol.BroadcastFlagsFromMessages(msgs)
	if err != nil {
		t.Fatalf("BroadcastFlagsFromMessages: %v", err)
	}
	if got != flags {
		t.Fatalf("flags = %#x, want %#x", got, flags)
	}
}

func TestSystemStateGetData(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.SystemStateGetData())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	state, err := protocol.SystemStateFromMessages(msgs)
	if err != nil {
		t.Fatalf("SystemStateFromMessages: %v", err)
	}
	if got := protocol.FormatXStatusFlags(state.CentralState); got != "ok" {
		t.Fatalf("CentralState = %q, want ok", got)
	}
	if len(msgs[0].Data) < 16 {
		t.Fatalf("system state payload len = %d, want >= 16", len(msgs[0].Data))
	}
}

func TestLogoffClearsClientSession(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const flags uint32 = 0x103
	if err := c.Send(ctx, protocol.SetBroadcastFlags(flags)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}
	if err := c.Send(ctx, protocol.Logoff()); err != nil {
		t.Fatalf("Logoff: %v", err)
	}

	msgs, err := c.Call(ctx, protocol.GetBroadcastFlags())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	got, err := protocol.BroadcastFlagsFromMessages(msgs)
	if err != nil {
		t.Fatalf("BroadcastFlagsFromMessages: %v", err)
	}
	if got != 0 {
		t.Fatalf("flags after logoff = %#x, want 0", got)
	}
}

func TestGetXFirmware(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetXFirmware())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	fw, err := protocol.XFirmwareFromMessages(msgs)
	if err != nil {
		t.Fatalf("XFirmwareFromMessages: %v", err)
	}
	if got := protocol.FormatXFirmwareVersion(fw); got == "" {
		t.Fatal("expected non-empty firmware version")
	}
}

func TestGetXStatus(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetXStatus())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	status, err := protocol.XStatusFromMessages(msgs)
	if err != nil {
		t.Fatalf("XStatusFromMessages: %v", err)
	}
	if got := protocol.FormatXStatusFlags(status.CentralState); got == "" {
		t.Fatal("expected non-empty central state")
	}
}

func TestSetTrackPower(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.SetTrackPower(false))
	if err != nil {
		t.Fatalf("SetTrackPower(off): %v", err)
	}
	off, err := protocol.TrackPowerFromMessages(msgs)
	if err != nil {
		t.Fatalf("TrackPowerFromMessages(off): %v", err)
	}
	if off {
		t.Fatal("expected track power off")
	}

	msgs, err = c.Call(ctx, protocol.GetXStatus())
	if err != nil {
		t.Fatalf("GetXStatus after off: %v", err)
	}
	xstatus, err := protocol.XStatusFromMessages(msgs)
	if err != nil {
		t.Fatalf("XStatusFromMessages: %v", err)
	}
	if got := protocol.FormatXStatusFlags(xstatus.CentralState); got != "track voltage off" {
		t.Fatalf("CentralState = %q, want track voltage off", got)
	}

	msgs, err = c.Call(ctx, protocol.SetTrackPower(true))
	if err != nil {
		t.Fatalf("SetTrackPower(on): %v", err)
	}
	on, err := protocol.TrackPowerFromMessages(msgs)
	if err != nil {
		t.Fatalf("TrackPowerFromMessages(on): %v", err)
	}
	if !on {
		t.Fatal("expected track power on")
	}

	msgs, err = c.Call(ctx, protocol.SystemStateGetData())
	if err != nil {
		t.Fatalf("SystemStateGetData: %v", err)
	}
	state, err := protocol.SystemStateFromMessages(msgs)
	if err != nil {
		t.Fatalf("SystemStateFromMessages: %v", err)
	}
	if got := protocol.FormatXStatusFlags(state.CentralState); got != "ok" {
		t.Fatalf("system state CentralState = %q, want ok", got)
	}
}

func TestBroadcastSystemState(t *testing.T) {
	srv, addr := startTestServer(t)

	c, err := client.DialLocal(addr, 42110)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, protocol.SetBroadcastFlags(protocol.BroadcastFlagSystemState)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}
	if _, err := c.Call(ctx, protocol.GetBroadcastFlags()); err != nil {
		t.Fatalf("GetBroadcastFlags: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		srv.BroadcastSystemState()
	}()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if !containsSystemStateBroadcast(msgs) {
		t.Fatalf("expected LAN_SYSTEMSTATE_DATACHANGED broadcast, got %#v", msgs)
	}
}

func TestTrackPowerChangeBroadcastsToPeer(t *testing.T) {
	_, addr := startTestServer(t)

	actor, err := client.DialLocal(addr, 42111)
	if err != nil {
		t.Fatalf("Dial actor: %v", err)
	}
	defer actor.Close()

	observer, err := client.DialLocal(addr, 42112)
	if err != nil {
		t.Fatalf("Dial observer: %v", err)
	}
	defer observer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	flags := protocol.BroadcastFlagXpressNet | protocol.BroadcastFlagSystemState
	if err := observer.Send(ctx, protocol.SetBroadcastFlags(flags)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}
	if _, err := observer.Call(ctx, protocol.GetBroadcastFlags()); err != nil {
		t.Fatalf("GetBroadcastFlags: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		if _, err := actor.Call(ctx, protocol.SetTrackPower(false)); err != nil {
			t.Errorf("SetTrackPower: %v", err)
		}
	}()

	var collected []protocol.Message
	for len(collected) == 0 || (!containsTrackPowerBC(collected, false) || !containsSystemStateBroadcast(collected)) {
		if readCtx.Err() != nil {
			t.Fatalf("timed out waiting for peer broadcasts, got %#v", collected)
		}
		packetCtx, packetCancel := context.WithTimeout(readCtx, 500*time.Millisecond)
		msgs, err := observer.ReadPacket(packetCtx)
		packetCancel()
		if err != nil {
			if readCtx.Err() != nil {
				t.Fatalf("timed out waiting for peer broadcasts, got %#v", collected)
			}
			continue
		}
		collected = append(collected, msgs...)
	}

	if !containsTrackPowerBC(collected, false) {
		t.Fatalf("expected LAN_X_BC_TRACK_POWER_OFF broadcast, got %#v", collected)
	}
	if !containsSystemStateBroadcast(collected) {
		t.Fatalf("expected LAN_SYSTEMSTATE_DATACHANGED broadcast, got %#v", collected)
	}
}

func TestTrackPowerChangeDoesNotEchoToRequester(t *testing.T) {
	_, addr := startTestServer(t)

	c, err := client.DialLocal(addr, 42113)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, protocol.SetBroadcastFlags(protocol.BroadcastFlagXpressNet)); err != nil {
		t.Fatalf("SetBroadcastFlags: %v", err)
	}
	if _, err := c.Call(ctx, protocol.GetBroadcastFlags()); err != nil {
		t.Fatalf("GetBroadcastFlags: %v", err)
	}

	if _, err := c.Call(ctx, protocol.SetTrackPower(false)); err != nil {
		t.Fatalf("SetTrackPower: %v", err)
	}

	collectCtx, collectCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer collectCancel()

	msgs, err := c.Collect(collectCtx)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if containsTrackPowerBC(msgs, false) {
		t.Fatalf("requester should not receive duplicate track power broadcast, got %#v", msgs)
	}
}

func containsSystemStateBroadcast(msgs []protocol.Message) bool {
	for _, msg := range msgs {
		if protocol.IsSystemStateDataChanged(msg) {
			return true
		}
	}
	return false
}

func containsTrackPowerBC(msgs []protocol.Message, wantOn bool) bool {
	for _, msg := range msgs {
		if msg.Header != protocol.HeaderLANX {
			continue
		}
		on, err := protocol.ParseTrackPowerBC(msg.Data)
		if err != nil {
			continue
		}
		if on == wantOn {
			return true
		}
	}
	return false
}
