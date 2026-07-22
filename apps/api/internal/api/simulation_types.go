package api

// SimulateCANRequest registers one or more CAN detectors on a simulator device.
type SimulateCANRequest struct {
	SimulationName string                  `json:"simulationName,omitempty"`
	CANDetectors   []SimulationCANDetector `json:"canDetectors"`
}

// SimulationCANDetector is a CAN occupancy detector module on a simulator.
type SimulationCANDetector struct {
	NetID         uint16 `json:"netID"`
	Name          string `json:"name,omitempty"`
	ModuleAddress uint16 `json:"moduleAddress,omitempty"`
	PortCount     uint16 `json:"portCount,omitempty"`
}

// Simulation is the API view of a Simulation custom resource.
type Simulation struct {
	Name         string                  `json:"name"`
	Namespace    string                  `json:"namespace"`
	DeviceRef    string                  `json:"deviceRef"`
	CANDetectors []SimulationCANDetector `json:"canDetectors,omitempty"`
	Status       *SimulationStatus       `json:"status,omitempty"`
}

// SimulationStatus mirrors the Simulation CR status fields used by clients.
type SimulationStatus struct {
	Phase             string `json:"phase,omitempty"`
	Message           string `json:"message,omitempty"`
	AppliedCANDevices int32  `json:"appliedCANDevices,omitempty"`
}
