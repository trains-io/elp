package trace

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/trains-io/z21.go/protocol"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	DirectionRX = "<<"
	DirectionTX = ">>"
)

// Tracer writes human-readable LAN and gRPC traffic dumps.
type Tracer struct {
	mu  sync.Mutex
	out io.Writer
}

// New returns a tracer writing to out, or nil when out is nil.
func New(out io.Writer) *Tracer {
	if out == nil {
		return nil
	}
	return &Tracer{out: out}
}

// LogLAN dumps a Z21 LAN UDP payload.
func (t *Tracer) LogLAN(direction, endpoint string, raw []byte) {
	if t == nil || len(raw) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	msgs, err := protocol.ParseAll(raw)
	if err != nil || len(msgs) == 0 {
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", direction, endpoint, len(raw))
		writeHexdump(t.out, raw)
		return
	}

	datasets, err := wireDatasets(raw)
	if err != nil || len(datasets) == 0 {
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", direction, endpoint, len(raw))
		writeHexdump(t.out, raw)
		return
	}

	switch len(datasets) {
	case 1:
		ds := datasets[0]
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", direction, endpoint, len(raw))
		fmt.Fprintf(t.out, "   %s\n", datasetSummary(ds.msg, len(ds.wire)))
		writeHexdump(t.out, ds.wire)
	default:
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes, %d datasets)\n", direction, endpoint, len(raw), len(datasets))
		for i, ds := range datasets {
			fmt.Fprintf(t.out, "   [%d/%d] %s\n", i+1, len(datasets), datasetSummary(ds.msg, len(ds.wire)))
			writeIndentedHexdump(t.out, ds.offset, ds.wire)
		}
	}
}

// LogLANBroadcast dumps an unsolicited LAN broadcast.
func (t *Tracer) LogLANBroadcast(flag uint32, recipients []string, raw []byte) {
	if t == nil || len(raw) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	endpoint := formatBroadcastEndpoint(flag, recipients)
	msgs, err := protocol.ParseAll(raw)
	if err != nil || len(msgs) == 0 {
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", DirectionTX, endpoint, len(raw))
		writeHexdump(t.out, raw)
		return
	}

	datasets, err := wireDatasets(raw)
	if err != nil || len(datasets) == 0 {
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", DirectionTX, endpoint, len(raw))
		writeHexdump(t.out, raw)
		return
	}

	switch len(datasets) {
	case 1:
		ds := datasets[0]
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes)\n", DirectionTX, endpoint, len(raw))
		fmt.Fprintf(t.out, "   %s\n", datasetSummary(ds.msg, len(ds.wire)))
		writeHexdump(t.out, ds.wire)
	default:
		fmt.Fprintf(t.out, "LAN %s %s (%d bytes, %d datasets)\n", DirectionTX, endpoint, len(raw), len(datasets))
		for i, ds := range datasets {
			fmt.Fprintf(t.out, "   [%d/%d] %s\n", i+1, len(datasets), datasetSummary(ds.msg, len(ds.wire)))
			writeIndentedHexdump(t.out, ds.offset, ds.wire)
		}
	}
}

func formatBroadcastEndpoint(flag uint32, recipients []string) string {
	name := broadcastFlagName(flag)
	switch len(recipients) {
	case 0:
		return fmt.Sprintf("broadcast %s (0 recipients)", name)
	case 1:
		return fmt.Sprintf("broadcast %s -> %s", name, recipients[0])
	default:
		return fmt.Sprintf("broadcast %s -> %s", name, strings.Join(recipients, ", "))
	}
}

func broadcastFlagName(flag uint32) string {
	for _, f := range protocol.KnownBroadcastFlags {
		if f.Value == flag {
			return f.Name
		}
	}
	return protocol.FormatBroadcastFlags(flag)
}

// LogGRPC dumps an incoming gRPC request.
func (t *Tracer) LogGRPC(method string, req proto.Message) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	fmt.Fprintf(t.out, "gRPC << %s\n", method)
	if req == nil {
		return
	}
	if b, err := protojson.Marshal(req); err == nil {
		fmt.Fprintf(t.out, "   %s\n", string(b))
		return
	}
	fmt.Fprintf(t.out, "   %v\n", req)
}

type wireDataset struct {
	msg    protocol.Message
	offset int
	wire   []byte
}

func wireDatasets(raw []byte) ([]wireDataset, error) {
	var out []wireDataset
	rest := raw
	offset := 0
	for len(rest) > 0 {
		msg, tail, err := protocol.Unmarshal(rest)
		if err != nil {
			return nil, err
		}
		consumed := len(rest) - len(tail)
		out = append(out, wireDataset{
			msg:    msg,
			offset: offset,
			wire:   rest[:consumed],
		})
		offset += consumed
		rest = tail
	}
	return out, nil
}

func datasetSummary(msg protocol.Message, wireLen int) string {
	name := protocol.MessageName(msg)
	if wireLen > 0 {
		return fmt.Sprintf("%s (%d bytes)", name, wireLen)
	}
	return name
}

func writeHexdump(w io.Writer, data []byte) {
	writeHexdumpAt(w, 0, data, "")
}

func writeIndentedHexdump(w io.Writer, baseOffset int, data []byte) {
	writeHexdumpAt(w, baseOffset, data, "       ")
}

func writeHexdumpAt(w io.Writer, baseOffset int, data []byte, indent string) {
	const width = 16
	for offset := 0; offset < len(data); offset += width {
		end := offset + width
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]

		fmt.Fprintf(w, "%s%08x  ", indent, baseOffset+offset)
		for i := 0; i < width; i++ {
			if i < len(chunk) {
				fmt.Fprintf(w, "%02x ", chunk[i])
			} else {
				fmt.Fprint(w, "   ")
			}
			if i == 7 {
				fmt.Fprint(w, " ")
			}
		}
		fmt.Fprint(w, " |")
		for _, b := range chunk {
			if b >= 32 && b < 127 {
				fmt.Fprintf(w, "%c", b)
			} else {
				fmt.Fprint(w, ".")
			}
		}
		fmt.Fprintln(w, "|")
	}
}
