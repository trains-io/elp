package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

const streamKeepAlive = 30 * time.Second

// DeviceStreamEvent is pushed to SSE clients.
type DeviceStreamEvent struct {
	Type   string   `json:"type"`
	Device Device   `json:"device,omitempty"`
	Name   string   `json:"name,omitempty"`
	Items  []Device `json:"items,omitempty"`
}

// StreamHub fans out device watch events to SSE subscribers per namespace.
type StreamHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan DeviceStreamEvent]struct{}
}

// NewStreamHub returns an empty stream hub.
func NewStreamHub() *StreamHub {
	return &StreamHub{
		subscribers: make(map[string]map[chan DeviceStreamEvent]struct{}),
	}
}

// Subscribe registers a subscriber for namespace updates. The returned cancel
// function must be called when the client disconnects.
func (h *StreamHub) Subscribe(namespace string) (chan DeviceStreamEvent, func()) {
	ch := make(chan DeviceStreamEvent, 32)

	h.mu.Lock()
	if h.subscribers[namespace] == nil {
		h.subscribers[namespace] = make(map[chan DeviceStreamEvent]struct{})
	}
	h.subscribers[namespace][ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		if subs, ok := h.subscribers[namespace]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, namespace)
			}
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

// Publish delivers an event to all subscribers in the namespace.
func (h *StreamHub) Publish(namespace string, event DeviceStreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers[namespace] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *DeviceHandler) streamDevices(w http.ResponseWriter, r *http.Request) {
	if h.StreamHub == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("device stream unavailable"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}

	namespace := chi.URLParam(r, "namespace")

	list, err := h.List(r.Context(), namespace)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := writeSSE(w, flusher, "snapshot", snapshotPayload(list.Items)); err != nil {
		return
	}

	events, unsubscribe := h.StreamHub.Subscribe(namespace)
	defer unsubscribe()

	keepAlive := time.NewTicker(streamKeepAlive)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, eventType := streamPayload(event)
			if err := writeSSE(w, flusher, eventType, payload); err != nil {
				return
			}
		}
	}
}

func snapshotPayload(items []Device) map[string]any {
	if items == nil {
		items = []Device{}
	}
	return map[string]any{
		"type":  "snapshot",
		"items": items,
	}
}

func streamPayload(event DeviceStreamEvent) (any, string) {
	switch event.Type {
	case "updated":
		return map[string]any{
			"type":   "updated",
			"device": event.Device,
		}, "updated"
	case "deleted":
		return map[string]any{
			"type": "deleted",
			"name": event.Name,
		}, "deleted"
	default:
		eventType := event.Type
		if eventType == "" {
			eventType = "message"
		}
		return event, eventType
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
