package canpool

import "fmt"

// MaxModuleAddress is the highest valid CAN detector module address.
const MaxModuleAddress uint16 = 255

// Config defines the allocatable address range for one Z21 command station.
type Config struct {
	// Start is the first auto-allocatable address (inclusive).
	Start uint16
	// End is the last auto-allocatable address (inclusive).
	End uint16
	// Reserved addresses are never chosen by Allocate; they may still be Claimed.
	Reserved []uint16
}

// Validate checks pool configuration.
func (c Config) Validate() error {
	if c.Start == 0 {
		return fmt.Errorf("canpool: start address must be >= 1")
	}
	if c.End > MaxModuleAddress {
		return fmt.Errorf("canpool: end address must be <= %d", MaxModuleAddress)
	}
	if c.Start > c.End {
		return fmt.Errorf("canpool: start address %d must be <= end address %d", c.Start, c.End)
	}

	seen := make(map[uint16]struct{}, len(c.Reserved))
	for _, addr := range c.Reserved {
		if addr == 0 {
			return fmt.Errorf("canpool: reserved address must be >= 1")
		}
		if addr < c.Start || addr > c.End {
			return fmt.Errorf("canpool: reserved address %d outside range %d-%d", addr, c.Start, c.End)
		}
		if _, dup := seen[addr]; dup {
			return fmt.Errorf("canpool: duplicate reserved address %d", addr)
		}
		seen[addr] = struct{}{}
	}
	return nil
}

func (c Config) reservedSet() map[uint16]struct{} {
	set := make(map[uint16]struct{}, len(c.Reserved))
	for _, addr := range c.Reserved {
		set[addr] = struct{}{}
	}
	return set
}

// Stats summarizes pool capacity and usage.
type Stats struct {
	RangeSize     int
	Reserved      int
	Allocated     int
	AutoAvailable int
}
