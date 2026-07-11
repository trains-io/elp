package client

import (
	"time"
)

type DeviceList struct {
	Items []Device `json:"items"`
}

type Device struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	Address        string         `json:"address"`
	NATS           NatsConfig     `json:"nats"`
	Gateway        *GatewayConfig `json:"gateway,omitempty"`
	BroadcastFlags uint32         `json:"broadcastFlags"`
	Status         *DeviceStatus  `json:"status,omitempty"`
}

type DeviceCreate struct {
	Name           string         `json:"name"`
	Address        string         `json:"address"`
	NATS           NatsConfig     `json:"nats"`
	Gateway        *GatewayConfig `json:"gateway,omitempty"`
	BroadcastFlags *uint32        `json:"broadcastFlags,omitempty"`
}

type NatsConfig struct {
	URL           string `json:"url"`
	SubjectPrefix string `json:"subjectPrefix,omitempty"`
}

type GatewayConfig struct {
	Image        string            `json:"image,omitempty"`
	HostNetwork  bool              `json:"hostNetwork,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

type DeviceStatus struct {
	Phase                     string     `json:"phase,omitempty"`
	GatewayDeployment         string     `json:"gatewayDeployment,omitempty"`
	GatewayReady              bool       `json:"gatewayReady,omitempty"`
	DeviceReachable           bool       `json:"deviceReachable,omitempty"`
	Degraded                  bool       `json:"degraded,omitempty"`
	HealthCheckFailures       int32      `json:"healthCheckFailures,omitempty"`
	LastSuccessfulHealthCheck *time.Time `json:"lastSuccessfulHealthCheck,omitempty"`
	LastSeen                  *time.Time `json:"lastSeen,omitempty"`
	SerialNumber              string     `json:"serialNumber,omitempty"`
	FirmwareVersion           string     `json:"firmwareVersion,omitempty"`
	HardwareType              string     `json:"hardwareType,omitempty"`
	XBusVersion               string     `json:"xbusVersion,omitempty"`
	XBusFirmwareVersion       string     `json:"xbusFirmwareVersion,omitempty"`
	LockCode                  *uint8     `json:"lockCode,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type StreamEvent struct {
	Type   string   `json:"type"`
	Items  []Device `json:"items,omitempty"`
	Device Device   `json:"device,omitempty"`
	Name   string   `json:"name,omitempty"`
}
