package detector

import (
	"testing"

	"github.com/trains-io/z21.go/protocol"
)

func TestSummarizeDiscovery(t *testing.T) {
	msgs := []protocol.Message{
		{
			Header: protocol.HeaderLANCANDetector,
			Data:   []byte{0x04, 0xdb, 0x1f, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00},
		},
		{
			Header: protocol.HeaderLANCANDetector,
			Data:   []byte{0x04, 0xdb, 0x1f, 0x00, 0x07, 0x01, 0x00, 0x01, 0x00, 0x00},
		},
		{
			Header: protocol.HeaderLANCANDetector,
			Data:   []byte{0x05, 0xdb, 0x18, 0x00, 0x02, 0x01, 0x00, 0x01, 0x00, 0x00},
		},
	}

	discovered, err := summarizeDiscovery(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered) != 2 {
		t.Fatalf("len(discovered) = %d, want 2", len(discovered))
	}

	byNetID := map[uint16]DiscoveredDetector{}
	for _, det := range discovered {
		byNetID[det.NetID] = det
	}

	db04 := byNetID[0xDB04]
	if db04.Addr != 31 || len(db04.Ports) != 2 {
		t.Fatalf("0xDB04 = %+v", db04)
	}
	if got := detectorName(0xDB04); got != "detector-db04" {
		t.Fatalf("detectorName() = %q", got)
	}
}

func TestProcessCANDetectorMessages(t *testing.T) {
	msgs := []protocol.Message{
		{
			Header: protocol.HeaderLANCANDetector,
			Data:   []byte{0x04, 0xdb, 0x1f, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00},
		},
	}

	discovered, err := summarizeDiscovery(msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered) != 1 || discovered[0].NetID != 0xDB04 || discovered[0].Addr != 31 {
		t.Fatalf("discovered = %#v", discovered)
	}
}

func TestNormalizeDetectorName(t *testing.T) {
	netID, ok := NormalizeDetectorName("detector-db04")
	if !ok || netID != 0xDB04 {
		t.Fatalf("netID = %#x ok=%v", netID, ok)
	}
}
