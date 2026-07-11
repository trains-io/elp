package events

import "testing"

func TestControlSubject(t *testing.T) {
	if got := ControlSubject("z21.default.main"); got != "z21.default.main.control" {
		t.Fatalf("ControlSubject() = %q", got)
	}
}
