package control

import (
	"context"
	"io"

	"github.com/trains-io/elp/apps/z21-sim/internal/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// Option configures a GRPCServer.
type Option func(*GRPCServer)

// WithTrace enables gRPC request dumps to out.
func WithTrace(out io.Writer) Option {
	return func(g *GRPCServer) {
		g.tracer = trace.New(out)
	}
}

func grpcTraceInterceptor(tr *trace.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if tr != nil {
			if msg, ok := req.(proto.Message); ok {
				tr.LogGRPC(info.FullMethod, msg)
			} else {
				tr.LogGRPC(info.FullMethod, nil)
			}
		}
		return handler(ctx, req)
	}
}
