package gateway

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/trains-io/elp/packages/events"
)

func subscribeControl(ctx context.Context, nc *nats.Conn, subjectPrefix string, session *z21Session, trace *messageTracer) (*nats.Subscription, error) {
	return nc.Subscribe(events.ControlSubject(subjectPrefix), func(msg *nats.Msg) {
		handleControlMessage(ctx, session, trace, msg.Data)
	})
}

func handleControlMessage(ctx context.Context, session *z21Session, trace *messageTracer, payload []byte) {
	var cmd events.ControlCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		slog.Warn("invalid control command", "error", err)
		return
	}

	switch cmd.Type {
	case events.CommandTypeSetBroadcastFlags:
		if err := session.applyBroadcastFlags(ctx, cmd.BroadcastFlags, trace); err != nil {
			slog.Warn("failed to apply broadcast flags", "error", err)
			return
		}
		slog.Info("applied broadcast flags", "flags", cmd.BroadcastFlags)
	default:
		slog.Warn("unknown control command", "type", cmd.Type)
	}
}
