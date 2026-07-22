package simulation

import "testing"

func TestParseNetID(t *testing.T) {
	tests := []struct {
		in   string
		want uint16
	}{
		{"56068", 0xDB04},
		{"0xDB04", 0xDB04},
		{"0xdb04", 0xDB04},
	}
	for _, tc := range tests {
		got, err := ParseNetID(tc.in)
		if err != nil {
			t.Fatalf("ParseNetID(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseNetID(%q) = %#x, want %#x", tc.in, got, tc.want)
		}
	}
}

func TestDetectorsFromNetIDsDefaults(t *testing.T) {
	detectors := DetectorsFromNetIDs(nil)
	if len(detectors) != 1 || detectors[0].NetID != 0xDB04 {
		t.Fatalf("detectors = %#v", detectors)
	}
}
