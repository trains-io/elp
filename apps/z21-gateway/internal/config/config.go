package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is loaded from environment variables set by the Z21Device controller.
type Config struct {
	Z21Address        string
	NATSURL           string
	NATSSubjectPrefix string
	DeviceName        string
	DeviceNamespace   string
	HealthAddr        string
	SkipStatus        bool
	LogMessages       bool
}

func Load() (Config, error) {
	cfg := Config{
		Z21Address:        os.Getenv("Z21_ADDRESS"),
		NATSURL:           os.Getenv("NATS_URL"),
		NATSSubjectPrefix: os.Getenv("NATS_SUBJECT_PREFIX"),
		DeviceName:        os.Getenv("Z21_DEVICE_NAME"),
		DeviceNamespace:   os.Getenv("Z21_DEVICE_NAMESPACE"),
		HealthAddr:        envOr("HEALTH_ADDR", ":8080"),
		SkipStatus:        envBool("Z21_GATEWAY_SKIP_STATUS"),
		LogMessages:       envBool("Z21_LOG_MESSAGES"),
	}

	switch {
	case cfg.Z21Address == "":
		return cfg, fmt.Errorf("Z21_ADDRESS is required")
	case cfg.NATSURL == "":
		return cfg, fmt.Errorf("NATS_URL is required")
	case cfg.NATSSubjectPrefix == "":
		return cfg, fmt.Errorf("NATS_SUBJECT_PREFIX is required")
	case cfg.DeviceName == "":
		return cfg, fmt.Errorf("Z21_DEVICE_NAME is required")
	case cfg.DeviceNamespace == "":
		return cfg, fmt.Errorf("Z21_DEVICE_NAMESPACE is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	v, _ := strconv.ParseBool(os.Getenv(key))
	return v
}
