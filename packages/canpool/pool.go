package canpool

import (
	"errors"
	"fmt"
	"sort"
)

// Sentinel errors for pool operations.
var (
	ErrAddressInUse         = errors.New("canpool: address already assigned")
	ErrAddressOutOfRange    = errors.New("canpool: address outside pool range")
	ErrAddressReserved      = errors.New("canpool: address is reserved for manual use")
	ErrNoAddressAvailable   = errors.New("canpool: no address available in range")
	ErrOwnerAlreadyAssigned = errors.New("canpool: owner already has an address")
	ErrOwnerNotFound        = errors.New("canpool: owner has no assignment")
)

// Assignment binds a stable owner identity to a module address.
// Owner is typically a Kubernetes namespaced name or NetID hex string.
type Assignment struct {
	Owner   string
	Address uint16
	NetID   uint16
}

// Pool tracks module address assignments for one command station.
type Pool struct {
	cfg         Config
	reserved    map[uint16]struct{}
	ownerByAddr map[uint16]string
	addrByOwner map[string]uint16
}

// New creates a pool from configuration.
func New(cfg Config) (*Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Pool{
		cfg:         cfg,
		reserved:    cfg.reservedSet(),
		ownerByAddr: make(map[uint16]string),
		addrByOwner: make(map[string]uint16),
	}, nil
}

// Config returns a copy of the pool configuration.
func (p *Pool) Config() Config {
	return p.cfg
}

// Rebuild replaces all assignments from external state (for example CANDetector CRs).
func (p *Pool) Rebuild(assignments []Assignment) error {
	p.ownerByAddr = make(map[uint16]string, len(assignments))
	p.addrByOwner = make(map[string]uint16, len(assignments))

	for _, a := range assignments {
		if a.Owner == "" {
			return fmt.Errorf("canpool: assignment owner is required")
		}
		if a.Address == 0 {
			return fmt.Errorf("canpool: assignment address is required for owner %q", a.Owner)
		}
		if !p.inRange(a.Address) {
			return fmt.Errorf("canpool: assignment address %d for owner %q: %w", a.Address, a.Owner, ErrAddressOutOfRange)
		}
		if other, ok := p.ownerByAddr[a.Address]; ok {
			return fmt.Errorf("canpool: duplicate assignment for address %d (%q and %q)", a.Address, other, a.Owner)
		}
		if prev, ok := p.addrByOwner[a.Owner]; ok {
			return fmt.Errorf("canpool: duplicate assignment for owner %q (addresses %d and %d)", a.Owner, prev, a.Address)
		}
		p.ownerByAddr[a.Address] = a.Owner
		p.addrByOwner[a.Owner] = a.Address
	}
	return nil
}

// Allocate picks the lowest free, non-reserved address for owner.
// Returns the existing address if owner is already assigned (idempotent).
func (p *Pool) Allocate(owner string) (uint16, error) {
	if owner == "" {
		return 0, fmt.Errorf("canpool: owner is required")
	}
	if addr, ok := p.addrByOwner[owner]; ok {
		return addr, nil
	}

	for addr := p.cfg.Start; addr <= p.cfg.End; addr++ {
		if p.isReserved(addr) {
			continue
		}
		if _, used := p.ownerByAddr[addr]; used {
			continue
		}
		p.ownerByAddr[addr] = owner
		p.addrByOwner[owner] = addr
		return addr, nil
	}
	return 0, ErrNoAddressAvailable
}

// Claim assigns a specific address to owner (for example adopting hardware config).
// Reserved addresses may be claimed explicitly.
func (p *Pool) Claim(owner string, addr uint16) error {
	if owner == "" {
		return fmt.Errorf("canpool: owner is required")
	}
	if addr == 0 {
		return fmt.Errorf("canpool: address must be >= 1")
	}
	if !p.inRange(addr) {
		return ErrAddressOutOfRange
	}
	if existing, ok := p.addrByOwner[owner]; ok {
		if existing == addr {
			return nil
		}
		return fmt.Errorf("canpool: owner %q already has address %d: %w", owner, existing, ErrOwnerAlreadyAssigned)
	}
	if other, used := p.ownerByAddr[addr]; used {
		return fmt.Errorf("canpool: address %d assigned to %q: %w", addr, other, ErrAddressInUse)
	}

	p.ownerByAddr[addr] = owner
	p.addrByOwner[owner] = addr
	return nil
}

// Release removes the assignment for owner.
func (p *Pool) Release(owner string) error {
	if owner == "" {
		return fmt.Errorf("canpool: owner is required")
	}
	addr, ok := p.addrByOwner[owner]
	if !ok {
		return ErrOwnerNotFound
	}
	delete(p.addrByOwner, owner)
	delete(p.ownerByAddr, addr)
	return nil
}

// ReleaseAddress removes whichever owner holds addr.
func (p *Pool) ReleaseAddress(addr uint16) error {
	owner, ok := p.ownerByAddr[addr]
	if !ok {
		return ErrOwnerNotFound
	}
	return p.Release(owner)
}

// OwnerOf returns the owner of addr, if any.
func (p *Pool) OwnerOf(addr uint16) (string, bool) {
	owner, ok := p.ownerByAddr[addr]
	return owner, ok
}

// AddressOf returns the address assigned to owner, if any.
func (p *Pool) AddressOf(owner string) (uint16, bool) {
	addr, ok := p.addrByOwner[owner]
	return addr, ok
}

// IsAvailable reports whether addr is in range and unassigned.
func (p *Pool) IsAvailable(addr uint16) bool {
	if !p.inRange(addr) {
		return false
	}
	_, used := p.ownerByAddr[addr]
	return !used
}

// IsAutoAllocatable reports whether Allocate may pick addr.
func (p *Pool) IsAutoAllocatable(addr uint16) bool {
	return p.IsAvailable(addr) && !p.isReserved(addr)
}

// Available returns sorted addresses Allocate may choose next.
func (p *Pool) Available() []uint16 {
	out := make([]uint16, 0, p.cfg.End-p.cfg.Start+1)
	for addr := p.cfg.Start; addr <= p.cfg.End; addr++ {
		if p.IsAutoAllocatable(addr) {
			out = append(out, addr)
		}
	}
	return out
}

// Assignments returns the current assignments sorted by address.
func (p *Pool) Assignments() []Assignment {
	out := make([]Assignment, 0, len(p.ownerByAddr))
	for addr, owner := range p.ownerByAddr {
		out = append(out, Assignment{Owner: owner, Address: addr})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

// Stats returns aggregate pool usage.
func (p *Pool) Stats() Stats {
	autoAvail := 0
	for addr := p.cfg.Start; addr <= p.cfg.End; addr++ {
		if p.IsAutoAllocatable(addr) {
			autoAvail++
		}
	}
	return Stats{
		RangeSize:     int(p.cfg.End-p.cfg.Start) + 1,
		Reserved:      len(p.reserved),
		Allocated:     len(p.ownerByAddr),
		AutoAvailable: autoAvail,
	}
}

func (p *Pool) inRange(addr uint16) bool {
	return addr >= p.cfg.Start && addr <= p.cfg.End
}

func (p *Pool) isReserved(addr uint16) bool {
	_, ok := p.reserved[addr]
	return ok
}
