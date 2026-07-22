package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/trains-io/elp/apps/z21-sim/internal/control"
	"github.com/trains-io/elp/apps/z21-sim/internal/server"
)

func main() {
	udpAddr := flag.String("addr", "0.0.0.0:21105", "UDP listen address")
	grpcAddr := flag.String("grpc-addr", "127.0.0.1:50051", "gRPC control listen address (empty disables)")
	traceFlag := flag.Bool("trace", false, "dump LAN and gRPC traffic to stderr")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var traceOpts []server.Option
	var grpcTraceOpts []control.Option
	if *traceFlag {
		traceOpts = append(traceOpts, server.WithTrace(os.Stderr))
		grpcTraceOpts = append(grpcTraceOpts, control.WithTrace(os.Stderr))
	}

	srv, err := server.New(*udpAddr, log, traceOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "z21-sim: %v\n", err)
		os.Exit(1)
	}
	defer srv.Close()

	log.Info("socket bound", "addr", srv.Addr())

	var grpcSrv *control.GRPCServer
	if *grpcAddr != "" {
		grpcSrv, err = control.NewGRPCServer(srv, *grpcAddr, log, grpcTraceOpts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "z21-sim: %v\n", err)
			os.Exit(1)
		}
		defer grpcSrv.Stop()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if grpcSrv != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := grpcSrv.Serve(); err != nil && !errors.Is(err, net.ErrClosed) {
				errCh <- fmt.Errorf("gRPC: %w", err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ctx); err != nil {
			errCh <- fmt.Errorf("UDP: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		if grpcSrv != nil {
			grpcSrv.Stop()
		}
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "z21-sim: %v\n", err)
		os.Exit(1)
	}

	wg.Wait()
}
