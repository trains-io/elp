package server

import (
	"context"
	"time"
)

func readDeadline(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Now().Add(200 * time.Millisecond)
}
