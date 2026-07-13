package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/namespaces/default/devices/basement" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req DeviceCreate
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Name != "basement" || req.Address != "192.168.0.42:21105" {
			t.Fatalf("unexpected body: %#v", req)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Device{
			Name:      "basement",
			Namespace: "default",
			Address:   req.Address,
			NATS:      req.NATS,
			Status:    &DeviceStatus{Phase: "Pending"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "default")
	device, err := c.CreateDevice(context.Background(), DeviceCreate{
		Name:    "basement",
		Address: "192.168.0.42:21105",
		NATS:    NatsConfig{URL: "nats://nats:4222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if device.Status == nil || device.Status.Phase != "Pending" {
		t.Fatalf("device = %#v", device)
	}
}

func TestGetDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/default/devices/basement" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Device{
			Name:      "basement",
			Namespace: "default",
			Address:   "192.168.0.42:21105",
			Status:    &DeviceStatus{Phase: "Running", GatewayReady: true},
		})
	}))
	defer srv.Close()

	device, err := New(srv.URL, "default").GetDevice(context.Background(), "basement")
	if err != nil {
		t.Fatal(err)
	}
	if device.Status.Phase != "Running" {
		t.Fatalf("phase = %q", device.Status.Phase)
	}
}

func TestDeleteDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/namespaces/default/devices/basement" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := New(srv.URL, "default").DeleteDevice(context.Background(), "basement"); err != nil {
		t.Fatal(err)
	}
}

func TestWatchDevicesSSE(t *testing.T) {
	body := strings.Join([]string{
		"event: snapshot",
		`data: {"type":"snapshot","items":[{"name":"basement","namespace":"default","address":"192.168.0.42:21105","nats":{"url":"nats://nats"},"broadcastFlags":257}]}`,
		"",
		"event: updated",
		`data: {"type":"updated","device":{"name":"basement","namespace":"default","address":"192.168.0.42:21105","nats":{"url":"nats://nats"},"broadcastFlags":257,"status":{"phase":"Running"}}}`,
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/default/devices/stream" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	var events []StreamEvent
	err := New(srv.URL, "default").WatchDevices(context.Background(), func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != "snapshot" || events[1].Device.Status.Phase != "Running" {
		t.Fatalf("events = %#v", events)
	}
}

func TestCreateDeviceSimulator(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/default/devices/dev-01" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		captured, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Device{
			Name:      "dev-01",
			Namespace: "default",
			Address:   "z21-sim-dev-01.elp.svc.cluster.local.:21105",
			NATS:      NatsConfig{URL: "nats://nats:4222"},
		})
	}))
	defer srv.Close()

	_, err := New(srv.URL, "default").CreateDevice(context.Background(), DeviceCreate{
		Name: "dev-01",
		Backend: &BackendCreate{
			Type:      "simulator",
			Simulator: &SimulatorCreate{},
		},
		NATS: NatsConfig{URL: "nats://nats:4222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(captured), `"name":"dev-01"`) {
		t.Fatalf("payload = %s", captured)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "not found"})
	}))
	defer srv.Close()

	_, err := New(srv.URL, "default").GetDevice(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadSSEKeepalive(t *testing.T) {
	input := ": keepalive\n\n" +
		"event: deleted\n" +
		`data: {"type":"deleted","name":"basement"}` + "\n\n"

	var got StreamEvent
	err := readSSE(strings.NewReader(input), func(event StreamEvent) error {
		got = event
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "deleted" || got.Name != "basement" {
		t.Fatalf("event = %#v", got)
	}
}

func ExampleClient_devicesPath() {
	c := New("http://localhost:8080", "default")
	fmt.Println(c.devicesPath())
	// Output: http://localhost:8080/api/v1/namespaces/default/devices
}
