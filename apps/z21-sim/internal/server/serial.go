package server

import (
	"encoding/binary"
	"time"

	"github.com/trains-io/z21.go/protocol"
)

// Decimal serials starting with 999 identify z21-sim instances (vs physical Z21 hardware).
const simSerialPrefix uint32 = 999

func makeSimulatorSerialNumber(now time.Time) uint32 {
	epochSeconds := uint32(now.Unix())
	return simSerialPrefix*1_000_000 + (epochSeconds % 1_000_000)
}

func (s *Server) serialReply() protocol.Message {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, s.serial)
	return protocol.Message{Header: protocol.HeaderLANGetSerialNumber, Data: data}
}

func (s *Server) codeReply() protocol.Message {
	return protocol.Message{Header: protocol.HeaderLANGetCode, Data: []byte{protocol.CodeNoLock}}
}
