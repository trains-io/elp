package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/trains-io/elp/apps/elpctl/internal/client"
)

func TestDeviceWatchWriterRedrawsInPlace(t *testing.T) {
	var buf bytes.Buffer
	w := &deviceWatchWriter{
		out:   &buf,
		isTTY: func(io.Writer) bool { return true },
	}

	items := []client.Device{{
		Name:      "dev-01",
		Namespace: "default",
		Address:   "z21-sim-dev-01.elp.svc.cluster.local.:21105",
		Status:    &client.DeviceStatus{Phase: "Starting", GatewayReady: false},
	}}
	if err := w.writeSnapshot(items); err != nil {
		t.Fatal(err)
	}
	first := buf.String()
	if w.lines == 0 {
		t.Fatal("expected line count after first render")
	}

	items[0].Status.Phase = "Running"
	items[0].Status.GatewayReady = true
	buf.Reset()
	if err := w.writeUpdated(items[0]); err != nil {
		t.Fatal(err)
	}
	second := buf.String()
	if !strings.HasPrefix(second, strings.Repeat("\033[F", w.lines)) {
		t.Fatalf("expected cursor-up redraw prefix, got %q", second)
	}
	if strings.Count(second, "NAME    NAMESPACE") != 1 {
		t.Fatalf("expected single table header in redraw, got %q", second)
	}
	if !strings.Contains(first, "Starting") {
		t.Fatalf("first render missing Starting: %q", first)
	}
	if !strings.Contains(second, "Running") {
		t.Fatalf("second render missing Running: %q", second)
	}
}

func TestDeviceWatchWriterNonTTYAppends(t *testing.T) {
	var buf bytes.Buffer
	w := &deviceWatchWriter{
		out:   &buf,
		isTTY: func(io.Writer) bool { return false },
	}

	device := client.Device{
		Name:      "dev-01",
		Namespace: "default",
		Address:   "z21-sim-dev-01.elp.svc.cluster.local.:21105",
		Status:    &client.DeviceStatus{Phase: "Starting"},
	}
	if err := w.writeSnapshot([]client.Device{device}); err != nil {
		t.Fatal(err)
	}
	device.Status.Phase = "Running"
	if err := w.writeUpdated(device); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "NAME    NAMESPACE") != 2 {
		t.Fatalf("expected appended output without redraw, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "\033[F") {
		t.Fatal("non-tty output must not use cursor controls")
	}
}

func TestDeviceTableLineCount(t *testing.T) {
	table := "NAME\tNAMESPACE\nfoo\tdefault\n"
	if got := deviceTableLineCount(table); got != 2 {
		t.Fatalf("line count = %d, want 2", got)
	}
}
