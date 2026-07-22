package server

import (
	"net"
	"testing"
	"time"

	"github.com/trains-io/z21.go/protocol"
)

func TestBroadcastSystemStateUDP(t *testing.T) {
	recvConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP recv: %v", err)
	}
	defer recvConn.Close()

	clientAddr := recvConn.LocalAddr().(*net.UDPAddr)

	srv, err := New("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()

	srv.mu.Lock()
	srv.clients[clientAddr.String()] = &clientSession{
		addr:           clientAddr,
		broadcastFlags: protocol.BroadcastFlagSystemState,
	}
	srv.mu.Unlock()

	srv.BroadcastSystemState()

	buf := make([]byte, 1472)
	_ = recvConn.SetReadDeadline(time.Now().Add(time.Second))
	n, from, err := recvConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUDP: %v", err)
	}
	if from.Port != srv.Addr().(*net.UDPAddr).Port {
		t.Fatalf("source port = %d, want %d", from.Port, srv.Addr().(*net.UDPAddr).Port)
	}

	msgs, err := protocol.ParseAll(buf[:n])
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if !containsSystemState(msgs) {
		t.Fatalf("expected system state broadcast, got %#v", msgs)
	}
}

func containsSystemState(msgs []protocol.Message) bool {
	for _, msg := range msgs {
		if protocol.IsSystemStateDataChanged(msg) {
			return true
		}
	}
	return false
}
