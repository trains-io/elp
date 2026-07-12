package gateway

import (
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/trains-io/z21.go/protocol"
)

type messageTracer struct {
	enabled bool
}

func newMessageTracer(enabled bool) *messageTracer {
	return &messageTracer{enabled: enabled}
}

func (t *messageTracer) tx(msgs ...protocol.Message) {
	t.log("tx", msgs...)
}

func (t *messageTracer) rx(msgs ...protocol.Message) {
	t.log("rx", msgs...)
}

func (t *messageTracer) log(direction string, msgs ...protocol.Message) {
	if t == nil || !t.enabled {
		return
	}
	for _, msg := range msgs {
		attrs := []any{
			"direction", direction,
			"message", protocol.MessageName(msg),
			"header", protocol.HeaderName(msg.Header),
			"header_hex", formatHeaderHex(msg.Header),
			"data_len", len(msg.Data),
		}
		if len(msg.Data) > 0 {
			attrs = append(attrs, "data_hex", hex.EncodeToString(msg.Data))
		}
		slog.Info("z21 message", attrs...)
	}
}

func formatHeaderHex(header uint16) string {
	return fmt.Sprintf("0x%04x", header)
}
