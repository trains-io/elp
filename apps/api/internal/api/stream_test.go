package api

import (
	"testing"
	"time"
)

func TestSnapshotPayload(t *testing.T) {
	payload := snapshotPayload(nil)
	if payload["type"] != "snapshot" {
		t.Fatalf("unexpected type: %#v", payload["type"])
	}
	items, ok := payload["items"].([]Device)
	if !ok || len(items) != 0 {
		t.Fatalf("expected empty items slice, got %#v", payload["items"])
	}
}

func TestStreamPayloadUpdated(t *testing.T) {
	payload, eventType := streamPayload(DeviceStreamEvent{
		Type:   "updated",
		Device: Device{Name: "main", Namespace: "default", Address: "127.0.0.1:21105"},
	})
	if eventType != "updated" {
		t.Fatalf("eventType = %q", eventType)
	}
	m, ok := payload.(map[string]any)
	if !ok || m["device"] == nil {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestStreamHubPublishSubscribe(t *testing.T) {
	hub := NewStreamHub()
	ch, cancel := hub.Subscribe("default")
	defer cancel()

	device := Device{Name: "main", Namespace: "default", Address: "127.0.0.1:21105"}
	hub.Publish("default", DeviceStreamEvent{Type: "updated", Device: device})

	select {
	case event := <-ch:
		if event.Type != "updated" || event.Device.Name != "main" {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestStreamHubNamespaceIsolation(t *testing.T) {
	hub := NewStreamHub()
	ch, cancel := hub.Subscribe("default")
	defer cancel()

	hub.Publish("other", DeviceStreamEvent{Type: "deleted", Name: "x"})

	select {
	case <-ch:
		t.Fatal("expected no event for other namespace")
	case <-time.After(50 * time.Millisecond):
	}
}
