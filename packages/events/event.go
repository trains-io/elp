package events

import "time"

// Event is a Z21 LAN dataset published to NATS.
type Event struct {
	Timestamp         time.Time `json:"timestamp"`
	DeviceName        string    `json:"deviceName"`
	DeviceNamespace   string    `json:"deviceNamespace"`
	Header            uint16    `json:"header"`
	Data              []byte    `json:"data"`
	SubjectPrefix     string    `json:"subjectPrefix"`
}

// EventsSubject returns the NATS subject for Z21 event streams.
func EventsSubject(prefix string) string {
	return prefix + ".events"
}
