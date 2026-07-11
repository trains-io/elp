package events

// Control command types published to the gateway control subject.
const (
	CommandTypeSetBroadcastFlags = "setBroadcastFlags"
)

// ControlCommand is a JSON command sent to a device gateway over NATS.
type ControlCommand struct {
	Type            string `json:"type"`
	DeviceName      string `json:"deviceName,omitempty"`
	DeviceNamespace string `json:"deviceNamespace,omitempty"`
	BroadcastFlags  uint32 `json:"broadcastFlags,omitempty"`
}

// ControlSubject returns the NATS subject for gateway control commands.
func ControlSubject(prefix string) string {
	return prefix + ".control"
}
