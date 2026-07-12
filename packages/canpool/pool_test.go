package canpool_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/trains-io/elp/packages/canpool"
)

func testConfig() canpool.Config {
	return canpool.Config{
		Start:    10,
		End:      20,
		Reserved: []uint16{15},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     canpool.Config
		wantErr string
	}{
		{
			name: "valid",
			cfg:  testConfig(),
		},
		{
			name:    "start zero",
			cfg:     canpool.Config{Start: 0, End: 10},
			wantErr: "start address must be >= 1",
		},
		{
			name:    "end too high",
			cfg:     canpool.Config{Start: 1, End: 300},
			wantErr: "end address must be <= 255",
		},
		{
			name:    "start after end",
			cfg:     canpool.Config{Start: 30, End: 20},
			wantErr: "start address 30 must be <= end address 20",
		},
		{
			name:    "reserved outside range",
			cfg:     canpool.Config{Start: 10, End: 20, Reserved: []uint16{5}},
			wantErr: "reserved address 5 outside range",
		},
		{
			name:    "duplicate reserved",
			cfg:     canpool.Config{Start: 10, End: 20, Reserved: []uint16{12, 12}},
			wantErr: "duplicate reserved address 12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestAllocateFirstFitSkipsReserved(t *testing.T) {
	p := newPool(t, testConfig())

	addr, err := p.Allocate("detector-a")
	if err != nil {
		t.Fatal(err)
	}
	if addr != 10 {
		t.Fatalf("first allocate = %d, want 10", addr)
	}

	addr, err = p.Allocate("detector-b")
	if err != nil {
		t.Fatal(err)
	}
	if addr != 11 {
		t.Fatalf("second allocate = %d, want 11", addr)
	}

	// Fill 12-14, skip reserved 15, then 16.
	for _, owner := range []string{"c", "d", "e"} {
		if _, err := p.Allocate(owner); err != nil {
			t.Fatalf("allocate %s: %v", owner, err)
		}
	}
	addr, err = p.Allocate("detector-f")
	if err != nil {
		t.Fatal(err)
	}
	if addr != 16 {
		t.Fatalf("after reserved skip = %d, want 16", addr)
	}
}

func TestAllocateIdempotent(t *testing.T) {
	p := newPool(t, testConfig())

	first, err := p.Allocate("detector-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.Allocate("detector-a")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("idempotent allocate = %d and %d", first, second)
	}
}

func TestAllocateExhausted(t *testing.T) {
	p := newPool(t, canpool.Config{Start: 1, End: 2})

	if _, err := p.Allocate("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Allocate("b"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Allocate("c"); !errors.Is(err, canpool.ErrNoAddressAvailable) {
		t.Fatalf("Allocate() error = %v, want ErrNoAddressAvailable", err)
	}
}

func TestClaimAndRelease(t *testing.T) {
	p := newPool(t, canpool.Config{Start: 50, End: 70})

	if err := p.Claim("detector-a", 60); err != nil {
		t.Fatal(err)
	}
	if err := p.Claim("detector-b", 60); !errors.Is(err, canpool.ErrAddressInUse) {
		t.Fatalf("Claim() error = %v, want ErrAddressInUse", err)
	}

	addr, ok := p.AddressOf("detector-a")
	if !ok || addr != 60 {
		t.Fatalf("AddressOf() = %d, %v", addr, ok)
	}

	if err := p.Release("detector-a"); err != nil {
		t.Fatal(err)
	}
	if !p.IsAvailable(60) {
		t.Fatal("expected address 60 to be available after release")
	}
}

func TestClaimOutOfRange(t *testing.T) {
	p := newPool(t, testConfig())
	if err := p.Claim("detector-a", 60); !errors.Is(err, canpool.ErrAddressOutOfRange) {
		t.Fatalf("Claim() error = %v, want ErrAddressOutOfRange", err)
	}
}

func TestClaimReservedAddress(t *testing.T) {
	p := newPool(t, testConfig())

	if err := p.Claim("legacy", 15); err != nil {
		t.Fatalf("Claim() reserved address: %v", err)
	}
	if _, err := p.Allocate("new"); err != nil {
		t.Fatal(err)
	}
	if got := p.Available(); containsUint16(got, 15) {
		t.Fatalf("Available() includes reserved 15: %v", got)
	}
}

func TestRebuildDetectsConflicts(t *testing.T) {
	p := newPool(t, testConfig())

	err := p.Rebuild([]canpool.Assignment{
		{Owner: "a", Address: 12},
		{Owner: "b", Address: 12},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate assignment for address 12") {
		t.Fatalf("Rebuild() error = %v", err)
	}
}

func TestRebuildRestoreState(t *testing.T) {
	p := newPool(t, testConfig())

	if err := p.Rebuild([]canpool.Assignment{
		{Owner: "detector-0xdb04", Address: 12, NetID: 0xDB04},
		{Owner: "detector-0xdb05", Address: 13, NetID: 0xDB05},
	}); err != nil {
		t.Fatal(err)
	}

	addr, err := p.Allocate("detector-new")
	if err != nil {
		t.Fatal(err)
	}
	if addr != 10 {
		t.Fatalf("Allocate() after rebuild = %d, want 10", addr)
	}

	assignments := p.Assignments()
	if len(assignments) != 3 {
		t.Fatalf("Assignments() len = %d, want 3", len(assignments))
	}
}

func TestReleaseAddress(t *testing.T) {
	p := newPool(t, testConfig())

	if _, err := p.Allocate("detector-a"); err != nil {
		t.Fatal(err)
	}
	if err := p.ReleaseAddress(10); err != nil {
		t.Fatal(err)
	}
	if err := p.ReleaseAddress(10); !errors.Is(err, canpool.ErrOwnerNotFound) {
		t.Fatalf("ReleaseAddress() error = %v", err)
	}
}

func TestStats(t *testing.T) {
	p := newPool(t, testConfig())

	stats := p.Stats()
	if stats.RangeSize != 11 || stats.Reserved != 1 || stats.Allocated != 0 || stats.AutoAvailable != 10 {
		t.Fatalf("Stats() = %+v", stats)
	}

	if _, err := p.Allocate("a"); err != nil {
		t.Fatal(err)
	}
	stats = p.Stats()
	if stats.Allocated != 1 || stats.AutoAvailable != 9 {
		t.Fatalf("Stats() after allocate = %+v", stats)
	}
}

func newPool(t *testing.T, cfg canpool.Config) *canpool.Pool {
	t.Helper()
	p, err := canpool.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return p
}

func containsUint16(vals []uint16, want uint16) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}
