package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/trains-io/elp/packages/events"

	z21v1alpha1 "github.com/trains-io/elp/operators/z21-device/api/v1alpha1"
)

// Publisher sends control commands to device gateways over NATS.
type Publisher struct {
	defaultURL string
	mu         sync.Mutex
	conns      map[string]*nats.Conn
}

// NewPublisher returns a publisher. defaultURL is an optional override (API NATS_URL)
// used when the API cannot reach in-cluster NATS URLs from device specs.
func NewPublisher(defaultURL string) *Publisher {
	return &Publisher{
		defaultURL: defaultURL,
		conns:      make(map[string]*nats.Conn),
	}
}

func (p *Publisher) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, nc := range p.conns {
		nc.Close()
	}
	p.conns = make(map[string]*nats.Conn)
}

func (p *Publisher) PublishSetBroadcastFlags(_ context.Context, device *z21v1alpha1.Z21Device) error {
	if p == nil {
		return fmt.Errorf("nats publisher unavailable")
	}

	url := p.natsURL(device)
	if url == "" {
		return fmt.Errorf("device %s/%s has no nats url", device.Namespace, device.Name)
	}

	nc, err := p.conn(url)
	if err != nil {
		return err
	}

	cmd := events.ControlCommand{
		Type:            events.CommandTypeSetBroadcastFlags,
		DeviceName:      device.Name,
		DeviceNamespace: device.Namespace,
		BroadcastFlags:  device.BroadcastFlagsValue(),
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return nc.Publish(events.ControlSubject(device.SubjectPrefix()), payload)
}

func (p *Publisher) natsURL(device *z21v1alpha1.Z21Device) string {
	if p.defaultURL != "" {
		return p.defaultURL
	}
	return device.Spec.NATS.URL
}

func (p *Publisher) conn(url string) (*nats.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if nc, ok := p.conns[url]; ok && nc.IsConnected() {
		return nc, nil
	}
	if nc, ok := p.conns[url]; ok {
		nc.Close()
		delete(p.conns, url)
	}

	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", url, err)
	}
	p.conns[url] = nc
	return nc, nil
}
