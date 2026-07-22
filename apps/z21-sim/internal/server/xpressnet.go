package server

import (
	"net"

	"github.com/trains-io/z21.go/protocol"
)

const (
	xHeaderGetVersion      byte = 0x21
	xCommandGetStatus      byte = 0x24
	xHeaderStatusReply     byte = 0x62
	xStatusDB0             byte = 0x22
	xCommandSetTrackPowerOff byte = 0x80
	xCommandSetTrackPowerOn  byte = 0x81
	xCommandGetFirmware    byte = 0xF1
	xFirmwareDB0           byte = 0x0A
	xHeaderFirmwareReply   byte = 0xF3
	xHeaderXBusBC          byte = 0x61
	xBCDB0TrackPowerOff    byte = 0x00
	xBCDB0TrackPowerOn     byte = 0x01
	defaultXFirmwareMSB    byte = 0x01
	defaultXFirmwareLSB    byte = 0x20 // BCD 1.20
)

func (s *Server) handleLANX(remote *net.UDPAddr, data []byte) (protocol.Message, bool) {
	if len(data) < 2 {
		return protocol.Message{}, false
	}

	switch data[0] {
	case xCommandGetFirmware:
		if data[1] == xFirmwareDB0 {
			return s.xFirmwareReply(), true
		}
	case xHeaderGetVersion:
		switch data[1] {
		case xCommandGetStatus:
			return s.xStatusReply(), true
		case xCommandSetTrackPowerOff:
			if s.setTrackPower(false) {
				s.notifyTrackPowerChange(false, remote)
			}
			return s.trackPowerBCReply(false), true
		case xCommandSetTrackPowerOn:
			if s.setTrackPower(true) {
				s.notifyTrackPowerChange(true, remote)
			}
			return s.trackPowerBCReply(true), true
		}
	}

	return protocol.Message{}, false
}

func (s *Server) setTrackPower(on bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trackPower == on {
		return false
	}
	s.trackPower = on
	return true
}

func (s *Server) centralState() byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trackPower {
		return 0
	}
	return centralStateTrackVoltageOff
}

func (s *Server) xFirmwareReply() protocol.Message {
	payload := []byte{xHeaderFirmwareReply, xFirmwareDB0, defaultXFirmwareMSB, defaultXFirmwareLSB}
	return protocol.Message{Header: protocol.HeaderLANX, Data: appendLANXXOR(payload)}
}

func (s *Server) xStatusReply() protocol.Message {
	state := s.centralState()
	payload := []byte{xHeaderStatusReply, xStatusDB0, state}
	return protocol.Message{Header: protocol.HeaderLANX, Data: appendLANXXOR(payload)}
}

func (s *Server) trackPowerBCReply(on bool) protocol.Message {
	db0 := xBCDB0TrackPowerOff
	if on {
		db0 = xBCDB0TrackPowerOn
	}
	payload := []byte{xHeaderXBusBC, db0}
	return protocol.Message{Header: protocol.HeaderLANX, Data: appendLANXXOR(payload)}
}

func appendLANXXOR(data []byte) []byte {
	out := make([]byte, len(data)+1)
	copy(out, data)
	var x byte
	for _, b := range data {
		x ^= b
	}
	out[len(data)] = x
	return out
}
