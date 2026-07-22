package server

import (
	"encoding/binary"
	"fmt"

	"github.com/trains-io/z21.go/protocol"
)

const (
	defaultDetectorOccupancy = 0x0100 // free with voltage
	defaultBoosterVCCmV      = 10000
	defaultBoosterCurrentmA  = 100
)

// CANDeviceKind identifies a simulated zLink CAN device.
type CANDeviceKind int

const (
	CANDeviceDetector CANDeviceKind = iota + 1
	CANDeviceBooster
)

// CANDevice is a simulated CAN occupancy detector or booster.
type CANDevice struct {
	Kind       CANDeviceKind
	NetID      uint16
	Name       string
	ModuleAddr uint16
	Ports      []uint16 // occupancy Value1 per port (detector)
	OutputPort uint16
	State      uint16
	VCCmV      uint16
	CurrentmA  uint16
}

func newDetectorDevice(netID uint16, name string, moduleAddr, portCount uint16) CANDevice {
	if moduleAddr == 0 {
		moduleAddr = 1
	}
	if portCount == 0 {
		portCount = 1
	}
	ports := make([]uint16, portCount)
	for i := range ports {
		ports[i] = defaultDetectorOccupancy
	}
	return CANDevice{
		Kind:       CANDeviceDetector,
		NetID:      netID,
		Name:       name,
		ModuleAddr: moduleAddr,
		Ports:      ports,
	}
}

func newBoosterDevice(netID uint16, name string, outputPort uint16) CANDevice {
	if outputPort == 0 {
		outputPort = 1
	}
	return CANDevice{
		Kind:       CANDeviceBooster,
		NetID:      netID,
		Name:       name,
		OutputPort: outputPort,
		VCCmV:      defaultBoosterVCCmV,
		CurrentmA:  defaultBoosterCurrentmA,
	}
}

// RegisterCANDevice stores or replaces a CAN device in the simulator registry.
func (s *Server) RegisterCANDevice(device CANDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.canDevices == nil {
		s.canDevices = make(map[uint16]CANDevice)
	}
	s.canDevices[device.NetID] = device
}

// NotifyCANDeviceDetected registers a device and broadcasts its initial status.
func (s *Server) NotifyCANDeviceDetected(device CANDevice) int {
	s.RegisterCANDevice(device)
	return s.broadcastCANDeviceDetected(device)
}

func (s *Server) broadcastCANDeviceDetected(device CANDevice) int {
	switch device.Kind {
	case CANDeviceDetector:
		return s.broadcastMany(s.detectorReports(device), protocol.BroadcastFlagCANDetector, nil)
	case CANDeviceBooster:
		return s.broadcast(s.boosterSystemStateMessage(device), protocol.BroadcastFlagCANBooster, nil)
	default:
		return 0
	}
}

func (s *Server) handleCAN(msg protocol.Message) ([]protocol.Message, bool) {
	switch msg.Header {
	case protocol.HeaderLANCANDeviceGetDescription:
		return s.handleCANDeviceGetDescription(msg.Data)
	case protocol.HeaderLANCANDeviceSetDescription:
		return s.handleCANDeviceSetDescription(msg.Data)
	case protocol.HeaderLANCANDetector:
		return s.handleCANDetectorPoll(msg.Data)
	case protocol.HeaderLANCANMaintenance:
		s.handleCANMaintenance(msg.Data)
		return nil, true
	case protocol.HeaderLANCANBoosterSetTrackPower:
		return s.handleCANBoosterSetTrackPower(msg.Data)
	default:
		return nil, false
	}
}

func (s *Server) handleCANDeviceGetDescription(data []byte) ([]protocol.Message, bool) {
	if len(data) < 2 {
		return nil, false
	}
	netID := binary.LittleEndian.Uint16(data[0:2])
	device, ok := s.canDevice(netID)
	name := ""
	if ok {
		name = device.Name
	}
	return []protocol.Message{{
		Header: protocol.HeaderLANCANDeviceGetDescription,
		Data:   encodeCANDeviceDescription(netID, name),
	}}, true
}

func (s *Server) handleCANDeviceSetDescription(data []byte) ([]protocol.Message, bool) {
	if len(data) < 2+protocol.CANBoosterNameLen {
		return nil, false
	}
	netID := binary.LittleEndian.Uint16(data[0:2])
	name := parseCANDeviceName(data[2 : 2+protocol.CANBoosterNameLen])

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.canDevices == nil {
		s.canDevices = make(map[uint16]CANDevice)
	}
	device, ok := s.canDevices[netID]
	if !ok {
		device = CANDevice{NetID: netID, Name: name}
	} else {
		device.Name = name
	}
	s.canDevices[netID] = device
	return nil, true
}

func (s *Server) handleCANDetectorPoll(data []byte) ([]protocol.Message, bool) {
	if len(data) < 3 || data[0] != 0x00 {
		return nil, false
	}
	netID := binary.LittleEndian.Uint16(data[1:3])

	var devices []CANDevice
	if netID == protocol.CANDetectorPollAll {
		devices = s.detectorDevices()
	} else if device, ok := s.canDevice(netID); ok && device.Kind == CANDeviceDetector {
		devices = []CANDevice{device}
	}

	if len(devices) == 0 {
		return nil, true
	}

	reports := make([]protocol.Message, 0)
	for _, device := range devices {
		for _, report := range s.detectorReports(device) {
			reports = append(reports, report)
		}
	}
	return reports, true
}

func (s *Server) handleCANMaintenance(data []byte) {
	netID, moduleAddr, _, err := protocol.ParseCANMaintenanceSetAddress(data)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.canDevices[netID]
	if !ok || device.Kind != CANDeviceDetector {
		return
	}
	device.ModuleAddr = moduleAddr
	s.canDevices[netID] = device
}

func (s *Server) handleCANBoosterSetTrackPower(data []byte) ([]protocol.Message, bool) {
	if len(data) < 3 {
		return nil, false
	}
	netID := binary.LittleEndian.Uint16(data[0:2])
	power := protocol.CANBoosterTrackPower(data[2])

	s.mu.Lock()
	device, ok := s.canDevices[netID]
	if !ok || device.Kind != CANDeviceBooster {
		s.mu.Unlock()
		return nil, true
	}

	switch power {
	case protocol.CANBoosterTrackPowerActivateAll, protocol.CANBoosterTrackPowerActivateOut1, protocol.CANBoosterTrackPowerActivateOut2:
		device.State &^= protocol.CANBoosterStateTrackVoltageOff
	case protocol.CANBoosterTrackPowerDeactivateAll, protocol.CANBoosterTrackPowerDeactivateOut1, protocol.CANBoosterTrackPowerDeactivateOut2:
		device.State |= protocol.CANBoosterStateTrackVoltageOff
	}
	s.canDevices[netID] = device
	s.mu.Unlock()

	reply := s.boosterSystemStateMessage(device)
	return []protocol.Message{reply}, true
}

func (s *Server) canDevice(netID uint16) (CANDevice, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.canDevices[netID]
	return device, ok
}

func (s *Server) detectorDevices() []CANDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []CANDevice
	for _, device := range s.canDevices {
		if device.Kind == CANDeviceDetector {
			out = append(out, device)
		}
	}
	return out
}

func (s *Server) detectorReports(device CANDevice) []protocol.Message {
	reports := make([]protocol.Message, 0, len(device.Ports))
	for port, value := range device.Ports {
		reports = append(reports, protocol.Message{
			Header: protocol.HeaderLANCANDetector,
			Data: encodeCANDetectorReport(protocol.CANDetectorReport{
				NetID:  device.NetID,
				Addr:   device.ModuleAddr,
				Port:   uint8(port),
				Type:   protocol.CANDetectorTypeOccupancy,
				Value1: value,
			}),
		})
	}
	return reports
}

func (s *Server) boosterSystemStateMessage(device CANDevice) protocol.Message {
	data := make([]byte, 10)
	binary.LittleEndian.PutUint16(data[0:2], device.NetID)
	binary.LittleEndian.PutUint16(data[2:4], device.OutputPort)
	binary.LittleEndian.PutUint16(data[4:6], device.State)
	binary.LittleEndian.PutUint16(data[6:8], device.VCCmV)
	binary.LittleEndian.PutUint16(data[8:10], device.CurrentmA)
	return protocol.Message{Header: protocol.HeaderLANCANBoosterSystemState, Data: data}
}

func encodeCANDeviceDescription(netID uint16, name string) []byte {
	data := make([]byte, 2+protocol.CANBoosterNameLen)
	binary.LittleEndian.PutUint16(data[0:2], netID)
	copy(data[2:], name)
	return data
}

func encodeCANDetectorReport(report protocol.CANDetectorReport) []byte {
	data := make([]byte, 10)
	binary.LittleEndian.PutUint16(data[0:2], report.NetID)
	binary.LittleEndian.PutUint16(data[2:4], report.Addr)
	data[4] = report.Port
	data[5] = report.Type
	binary.LittleEndian.PutUint16(data[6:8], report.Value1)
	binary.LittleEndian.PutUint16(data[8:10], report.Value2)
	return data
}

func parseCANDeviceName(raw []byte) string {
	for i, b := range raw {
		if b == 0 {
			return string(raw[:i])
		}
	}
	return string(raw)
}

// BuildCANDevice constructs a CAN device from control-plane parameters.
func BuildCANDevice(netID uint32, kind CANDeviceKind, name string, moduleAddr, portCount, outputPort uint32) (CANDevice, error) {
	if netID == 0 || netID > 0xFFFF {
		return CANDevice{}, fmt.Errorf("z21-sim: net_id out of range")
	}
	switch kind {
	case CANDeviceDetector:
		return newDetectorDevice(uint16(netID), name, uint16(moduleAddr), uint16(portCount)), nil
	case CANDeviceBooster:
		return newBoosterDevice(uint16(netID), name, uint16(outputPort)), nil
	default:
		return CANDevice{}, fmt.Errorf("z21-sim: unsupported CAN device kind")
	}
}
