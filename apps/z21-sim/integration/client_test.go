//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/trains-io/z21.go/client"
	"github.com/trains-io/z21.go/protocol"
)

var (
	sharedTestServerAddr        string
	sharedTestServerStop        func()
	sharedTestServerUnavailable bool
)

func TestMain(m *testing.M) {
	addr, stop, err := startSharedTestServer()
	switch {
	case err == nil:
		sharedTestServerAddr = addr
		sharedTestServerStop = stop
	case isDockerSkippable(err):
		sharedTestServerUnavailable = true
	default:
		fmt.Fprintf(os.Stderr, "start z21-sim test server: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if sharedTestServerStop != nil {
		sharedTestServerStop()
	}
	os.Exit(code)
}

func requireDocker(t *testing.T) {
	t.Helper()
	if sharedTestServerUnavailable {
		t.Skip("docker not available; skipping integration test")
	}
	if sharedTestServerAddr == "" {
		t.Fatal("shared z21-sim test server not configured")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed; skipping integration test")
	}
}

func testServerAddr(t *testing.T) string {
	t.Helper()
	requireDocker(t)
	return sharedTestServerAddr
}

func startSharedTestServer() (addr string, terminate func(), err error) {
	if _, lookErr := exec.LookPath("docker"); lookErr != nil {
		return "", nil, errDockerUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(ctx, "docker", "info")
	out, dockerErr := cmd.CombinedOutput()
	cancel()
	if dockerErr != nil {
		return "", nil, fmt.Errorf("%w: %v\n%s", errDockerUnavailable, dockerErr, out)
	}

	ctx = context.Background()
	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{"21105/udp"},
		WaitingFor: wait.ForLog("socket bound").
			WithStartupTimeout(60 * time.Second),
	}

	switch {
	case os.Getenv("Z21_TESTSERVER_IMAGE") != "":
		req.Image = os.Getenv("Z21_TESTSERVER_IMAGE")
	case os.Getenv("Z21_TESTSERVER_DOCKERFILE") != "":
		req.FromDockerfile = testcontainers.FromDockerfile{
			Context:    os.Getenv("Z21_TESTSERVER_DOCKERFILE"),
			Dockerfile: "Dockerfile",
			KeepImage:  true,
		}
	default:
		return "", nil, fmt.Errorf("set Z21_TESTSERVER_IMAGE or Z21_TESTSERVER_DOCKERFILE")
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if isDockerInfraError(err) {
			return "", nil, fmt.Errorf("%w: %v", errDockerUnavailable, err)
		}
		return "", nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, err
	}

	port, err := container.MappedPort(ctx, "21105/udp")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, err
	}

	addr = normalizeTestServerHost(host) + ":" + port.Port()
	if err := waitForServerReady(addr); err != nil {
		_ = container.Terminate(ctx)
		return "", nil, err
	}

	return addr, func() {
		_ = container.Terminate(ctx)
	}, nil
}

func normalizeTestServerHost(host string) string {
	if host == "localhost" {
		return "127.0.0.1"
	}
	return host
}

func waitForServerReady(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var lastErr error
	for {
		c, err := client.Dial(addr)
		if err == nil {
			_, err = c.Call(ctx, protocol.GetHWInfo())
			_ = c.Close()
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("z21-sim not ready at %s: %w (last error: %v)", addr, ctx.Err(), lastErr)
			}
			return fmt.Errorf("z21-sim not ready at %s: %w", addr, ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestGetHWInfo(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetHWInfo())
	require.NoError(t, err)

	hw, err := protocol.HWInfoFromMessages(msgs)
	require.NoError(t, err)
	require.Equal(t, protocol.HwTypeZ21New, hw.HwType)
}

func TestGetSerialNumber(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetSerialNumber())
	require.NoError(t, err)

	serial, ok := protocol.SerialFromMessages(msgs)
	require.True(t, ok)
	require.NotEmpty(t, serial)
	require.Equal(t, "999", serial[:3])
}

func TestGetCode(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetCode())
	require.NoError(t, err)

	code, err := protocol.CodeFromMessages(msgs)
	require.NoError(t, err)
	require.Equal(t, protocol.CodeNoLock, code)
}

func TestSystemStateGetData(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.SystemStateGetData())
	require.NoError(t, err)

	_, err = protocol.SystemStateFromMessages(msgs)
	require.NoError(t, err)
}

func TestGetBroadcastFlags(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const flags uint32 = 0x103
	require.NoError(t, c.Send(ctx, protocol.SetBroadcastFlags(flags)))

	msgs, err := c.Call(ctx, protocol.GetBroadcastFlags())
	require.NoError(t, err)

	got, err := protocol.BroadcastFlagsFromMessages(msgs)
	require.NoError(t, err)
	require.Equal(t, flags, got)
}

func TestGetXFirmware(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetXFirmware())
	require.NoError(t, err)

	fw, err := protocol.XFirmwareFromMessages(msgs)
	require.NoError(t, err)
	require.NotEmpty(t, protocol.FormatXFirmwareVersion(fw))
}

func TestGetXStatus(t *testing.T) {
	addr := testServerAddr(t)

	c, err := client.Dial(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, err := c.Call(ctx, protocol.GetXStatus())
	require.NoError(t, err)

	status, err := protocol.XStatusFromMessages(msgs)
	require.NoError(t, err)
	require.NotEmpty(t, protocol.FormatXStatusFlags(status.CentralState))
}

var errDockerUnavailable = errors.New("docker unavailable")

func isDockerSkippable(err error) bool {
	return errors.Is(err, errDockerUnavailable) || isDockerInfraError(err)
}

func isDockerInfraError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "build image") ||
		strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "connect: connection refused") ||
		strings.Contains(msg, "Cannot connect to the Docker daemon")
}
