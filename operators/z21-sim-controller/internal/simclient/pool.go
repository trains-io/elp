package simclient

import "sync"

// Pool returns cached gRPC clients keyed by simulator control endpoint.
type Pool interface {
	ClientFor(addr string) Client
	Close() error
}

// ClientPool lazily dials and reuses gRPC clients per address.
type ClientPool struct {
	mu      sync.Mutex
	clients map[string]*GRPCClient
}

// NewPool creates an empty client pool.
func NewPool() *ClientPool {
	return &ClientPool{clients: make(map[string]*GRPCClient)}
}

func (p *ClientPool) ClientFor(addr string) Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	if client, ok := p.clients[addr]; ok {
		return client
	}
	client := NewGRPC(addr)
	p.clients[addr] = client
	return client
}

func (p *ClientPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for addr, client := range p.clients {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.clients, addr)
	}
	return firstErr
}
