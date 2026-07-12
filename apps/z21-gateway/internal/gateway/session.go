package gateway

import (
	"context"
	"sync"

	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

type z21Session struct {
	mu     sync.Mutex
	client *client.Client
	flags  uint32
}

func newZ21Session(initialFlags uint32) *z21Session {
	if initialFlags == 0 {
		initialFlags = protocol.DefaultBroadcastFlags
	}
	return &z21Session{flags: initialFlags}
}

func (s *z21Session) flagsValue() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flags
}

func (s *z21Session) setFlags(flags uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if flags == 0 {
		flags = protocol.DefaultBroadcastFlags
	}
	s.flags = flags
}

func (s *z21Session) attach(c *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = c
}

func (s *z21Session) detach() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = nil
}

func (s *z21Session) applyBroadcastFlags(ctx context.Context, flags uint32, trace *messageTracer) error {
	s.setFlags(flags)

	s.mu.Lock()
	c := s.client
	effective := s.flags
	s.mu.Unlock()

	if c == nil {
		return nil
	}
	return sendWithTimeout(ctx, c, protocol.SetBroadcastFlags(effective), trace)
}
