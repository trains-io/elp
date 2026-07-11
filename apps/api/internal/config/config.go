package config

import (
	"os"
)

type Config struct {
	Addr    string
	NATSURL string
}

func Load() Config {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		Addr:    addr,
		NATSURL: os.Getenv("NATS_URL"),
	}
}
