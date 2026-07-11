package main

import (
	"os"

	"github.com/trains-io/elp/apps/elpctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
