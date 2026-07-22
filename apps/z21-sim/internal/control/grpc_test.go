package control_test

import (
	"context"
	"testing"
	"time"

	controlv1 "github.com/trains-io/elp/apps/z21-sim/api/control/v1"
	"github.com/trains-io/elp/apps/z21-sim/internal/control"
	"github.com/trains-io/elp/apps/z21-sim/internal/server"
	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func startSimulator(t *testing.T) (*server.Server, string) {
	t.Helper()

	sim, err := server.New("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- sim.Serve(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("simulator did not stop cleanly")
		}
		_ = sim.Close()
	})

	return sim, sim.Addr().String()
}

func TestGRPCBroadcastSystemState(t *testing.T) {
	sim, udpAddr := startSimulator(t)

	grpcSrv, err := control.NewGRPCServer(sim, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("NewGRPCServer: %v", err)
	}
	t.Cleanup(grpcSrv.Stop)

	go func() {
		_ = grpcSrv.Serve()
	}()

	c, err := client.DialLocal(udpAddr, 42120)
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
		grpcCtx, grpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer grpcCancel()

		conn, err := grpc.NewClient(
			grpcSrv.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			t.Errorf("grpc.NewClient: %v", err)
			return
		}
		defer conn.Close()

		_, err = controlv1.NewSimulatorServiceClient(conn).BroadcastSystemState(grpcCtx, &controlv1.BroadcastSystemStateRequest{})
		if err != nil {
			t.Errorf("BroadcastSystemState: %v", err)
		}
	}()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if !containsSystemStateBroadcast(msgs) {
		t.Fatalf("expected system state broadcast, got %#v", msgs)
	}
}

func TestGRPCSetTrackPower(t *testing.T) {
	sim, udpAddr := startSimulator(t)

	grpcSrv, err := control.NewGRPCServer(sim, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("NewGRPCServer: %v", err)
	}
	t.Cleanup(grpcSrv.Stop)

	go func() {
		_ = grpcSrv.Serve()
	}()

	conn, err := grpc.NewClient(
		grpcSrv.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	grpcClient := controlv1.NewSimulatorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := grpcClient.SetTrackPower(ctx, &controlv1.SetTrackPowerRequest{On: false})
	if err != nil {
		t.Fatalf("SetTrackPower: %v", err)
	}
	if resp.GetOn() {
		t.Fatalf("resp.On = true, want false")
	}
	if !resp.GetChanged() {
		t.Fatal("resp.Changed = false, want true")
	}

	c, err := client.Dial(udpAddr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	msgs, err := c.Call(ctx, protocol.GetXStatus())
	if err != nil {
		t.Fatalf("GetXStatus: %v", err)
	}
	status, err := protocol.XStatusFromMessages(msgs)
	if err != nil {
		t.Fatalf("XStatusFromMessages: %v", err)
	}
	if got := protocol.FormatXStatusFlags(status.CentralState); got != "track voltage off" {
		t.Fatalf("CentralState = %q, want track voltage off", got)
	}
}

func TestGRPCNotifyCANDeviceDetected(t *testing.T) {
	sim, udpAddr := startSimulator(t)

	grpcSrv, err := control.NewGRPCServer(sim, "127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("NewGRPCServer: %v", err)
	}
	t.Cleanup(grpcSrv.Stop)

	go func() {
		_ = grpcSrv.Serve()
	}()

	c, err := client.DialLocal(udpAddr, 42131)
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
		grpcCtx, grpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer grpcCancel()

		conn, err := grpc.NewClient(
			grpcSrv.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			t.Errorf("grpc.NewClient: %v", err)
			return
		}
		defer conn.Close()

		_, err = controlv1.NewSimulatorServiceClient(conn).NotifyCANDeviceDetected(grpcCtx, &controlv1.NotifyCANDeviceDetectedRequest{
			NetId:      0xDB04,
			Kind:       controlv1.CANDeviceKind_CAN_DEVICE_KIND_DETECTOR,
			Name:       "Det 4",
			ModuleAddr: 60,
			PortCount:  1,
		})
		if err != nil {
			t.Errorf("NotifyCANDeviceDetected: %v", err)
		}
	}()

	msgs, err := c.ReadPacket(readCtx)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	reports, err := protocol.CANDetectorReportsFromMessages(msgs)
	if err != nil {
		t.Fatalf("CANDetectorReportsFromMessages: %v", err)
	}
	if len(reports) != 1 || reports[0].NetID != 0xDB04 || reports[0].Addr != 60 {
		t.Fatalf("reports = %+v", reports)
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
