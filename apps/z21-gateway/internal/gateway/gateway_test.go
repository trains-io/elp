package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestIsReadTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "context deadline",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "net timeout",
			err:  &net.OpError{Op: "read", Net: "udp", Err: timeoutError{}},
			want: true,
		},
		{
			name: "wrapped udp timeout",
			err:  fmt.Errorf("read z21 packet: %w", fmt.Errorf("z21 client: read: %w", &net.OpError{Op: "read", Net: "udp", Err: timeoutError{}})),
			want: true,
		},
		{
			name: "connection refused",
			err:  &net.OpError{Op: "read", Net: "udp", Err: errors.New("connection refused")},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isReadTimeout(tt.err); got != tt.want {
				t.Fatalf("isReadTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}
var _ interface{ Timeout() bool } = timeoutError{}

func TestReadTimeoutMatchesGatewayLog(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf(
		"read z21 packet: %w",
		fmt.Errorf("z21 client: read: %w", &net.OpError{
			Op:  "read",
			Net: "udp",
			Err: timeoutError{},
		}),
	)
	if !isReadTimeout(err) {
		t.Fatal("expected gateway read timeout error to be treated as transient")
	}
}

func TestWaitForRetryRespectsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitForRetry(ctx, time.Second) {
		t.Fatal("expected waitForRetry to return false when context is cancelled")
	}
}

func TestWaitForRetryWaits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	start := time.Now()
	if !waitForRetry(ctx, 10*time.Millisecond) {
		t.Fatal("expected waitForRetry to return true")
	}
	if time.Since(start) < 5*time.Millisecond {
		t.Fatal("expected waitForRetry to block until interval elapsed")
	}
}

func TestTimeoutErrorInterface(t *testing.T) {
	t.Parallel()

	var err error = timeoutError{}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatal("timeoutError should satisfy net.Error timeout check")
	}
}
