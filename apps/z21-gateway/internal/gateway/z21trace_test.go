package gateway

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/trains-io/z21.go/protocol"
)

func TestMessageTracerDisabled(t *testing.T) {
	trace := newMessageTracer(false)
	trace.tx(protocol.GetHWInfo())
	trace.rx(protocol.Message{Header: protocol.HeaderLANGetHWInfo, Data: []byte{1, 2, 3, 4}})
}

func TestMessageTracerEnabled(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	trace := newMessageTracer(true)
	trace.tx(protocol.GetHWInfo())
	trace.rx(protocol.Message{
		Header: protocol.HeaderLANGetHWInfo,
		Data:   []byte{0x01, 0x02, 0x00, 0x00, 0x43, 0x01, 0x00, 0x00},
	})

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(`"direction":"tx"`)) {
		t.Fatalf("expected tx log, got %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"direction":"rx"`)) {
		t.Fatalf("expected rx log, got %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`LAN_GET_HWINFO`)) {
		t.Fatalf("expected header name, got %s", out)
	}
}
