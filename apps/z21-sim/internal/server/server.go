package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/trains-io/elp/apps/z21-sim/internal/trace"
	"github.com/trains-io/z21.go/protocol"
)

const defaultFirmwareVersion uint32 = 0x00000120 // BCD 1.20

// Server implements a minimal Z21 LAN UDP API.
type Server struct {
	conn       *net.UDPConn
	log        *slog.Logger
	tracer     *trace.Tracer
	serial     uint32
	trackPower bool
	mu         sync.Mutex
	clients    map[string]*clientSession
	canDevices map[uint16]CANDevice
}

type clientSession struct {
	addr           *net.UDPAddr
	broadcastFlags uint32
}

// New binds a UDP listener on addr (for example "0.0.0.0:21105" or "127.0.0.1:0").
func New(addr string, log *slog.Logger, opts ...Option) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("z21-sim: resolve addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("z21-sim: listen: %w", err)
	}

	s := &Server{
		conn:       conn,
		log:        log,
		serial:     makeSimulatorSerialNumber(time.Now()),
		trackPower: true,
		clients:    make(map[string]*clientSession),
		canDevices: make(map[uint16]CANDevice),
	}
	applyOptions(s, opts)
	return s, nil
}

func applyOptions(s *Server, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
}

// Addr returns the bound local UDP address.
func (s *Server) Addr() net.Addr {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

// Close releases the UDP socket.
func (s *Server) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Serve handles incoming datagrams until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("z21-sim: not listening")
	}

	buf := make([]byte, 4096)
	for {
		if err := s.conn.SetReadDeadline(readDeadline(ctx)); err != nil {
			return fmt.Errorf("z21-sim: set read deadline: %w", err)
		}

		n, remote, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return fmt.Errorf("z21-sim: read: %w", err)
		}

		s.handlePacket(remote, buf[:n])
	}
}

func (s *Server) handlePacket(remote *net.UDPAddr, payload []byte) {
	if s.tracer != nil {
		s.tracer.LogLAN(trace.DirectionRX, remote.String(), payload)
	}

	msgs, err := protocol.ParseAll(payload)
	if err != nil {
		s.log.Debug("ignore malformed packet", "from", remote, "err", err)
		return
	}

	session := s.sessionFor(remote)
	var replies []protocol.Message

	for _, msg := range msgs {
		switch msg.Header {
		case protocol.HeaderLANGetHWInfo:
			replies = append(replies, s.hwInfoReply())
		case protocol.HeaderLANGetSerialNumber:
			replies = append(replies, s.serialReply())
		case protocol.HeaderLANGetCode:
			replies = append(replies, s.codeReply())
		case protocol.HeaderLANLogoff:
			s.removeClient(remote)
		case protocol.HeaderLANSetBroadcastFlags:
			if len(msg.Data) >= 4 {
				flags := binary.LittleEndian.Uint32(msg.Data)
				s.mu.Lock()
				session.broadcastFlags = flags
				s.mu.Unlock()
			}
		case protocol.HeaderLANGetBroadcastFlags:
			replies = append(replies, s.broadcastFlagsReply(session.broadcastFlags))
		case protocol.HeaderLANSystemStateGetData:
			replies = append(replies, s.systemStateReply())
		case protocol.HeaderLANX:
			if reply, ok := s.handleLANX(remote, msg.Data); ok {
				replies = append(replies, reply)
			} else {
				s.log.Debug("unhandled LAN_X", "from", remote, "data", msg.Data)
			}
		case protocol.HeaderLANCANDeviceGetDescription,
			protocol.HeaderLANCANDeviceSetDescription,
			protocol.HeaderLANCANDetector,
			protocol.HeaderLANCANMaintenance,
			protocol.HeaderLANCANBoosterSetTrackPower:
			if canReplies, ok := s.handleCAN(msg); ok {
				replies = append(replies, canReplies...)
			} else {
				s.log.Debug("unhandled CAN", "from", remote, "header", protocol.HeaderName(msg.Header))
			}
		default:
			s.log.Debug("unhandled request", "from", remote, "header", protocol.HeaderName(msg.Header))
		}
	}

	if len(replies) == 0 {
		return
	}
	s.writeMessages(remote, replies)
}

func (s *Server) writeMessages(remote *net.UDPAddr, msgs []protocol.Message) {
	out, err := protocol.MarshalAll(msgs...)
	if err != nil {
		s.log.Error("marshal reply", "err", err)
		return
	}

	if s.tracer != nil {
		s.tracer.LogLAN(trace.DirectionTX, remote.String(), out)
	}

	s.sendUDP(remote, out)
}

func (s *Server) sendUDP(remote *net.UDPAddr, out []byte) bool {
	if _, err := s.conn.WriteToUDP(out, remote); err != nil {
		s.log.Error("write udp", "to", remote, "err", err)
		return false
	}
	return true
}

func (s *Server) sessionFor(remote *net.UDPAddr) *clientSession {
	key := remote.String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.clients[key]; ok {
		c.addr = remote
		return c
	}

	c := &clientSession{addr: remote}
	s.clients[key] = c
	return c
}

func (s *Server) clientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *Server) removeClient(remote *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, remote.String())
}

func (s *Server) hwInfoReply() protocol.Message {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:4], protocol.HwTypeZ21New)
	binary.LittleEndian.PutUint32(data[4:8], defaultFirmwareVersion)
	return protocol.Message{Header: protocol.HeaderLANGetHWInfo, Data: data}
}
