package server

import (
	"io"

	"github.com/trains-io/elp/apps/z21-sim/internal/trace"
)

// Option configures a Server.
type Option func(*Server)

// WithTrace enables LAN traffic dumps to out.
func WithTrace(out io.Writer) Option {
	return func(s *Server) {
		s.tracer = trace.New(out)
	}
}
