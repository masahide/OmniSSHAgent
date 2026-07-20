package app

import (
	"errors"
	"testing"
)

func TestAggregate(t *testing.T) {
	if got := Aggregate(nil, []ComponentStatus{{Enabled: true, Running: true}}); got != StateRunning {
		t.Fatal(got)
	}
	if got := Aggregate(nil, []ComponentStatus{{Enabled: true, Error: errors.New("failed")}}); got != StateDegraded {
		t.Fatal(got)
	}
	if got := Aggregate(errors.New("bad config"), nil); got != StateConfigurationError {
		t.Fatal(got)
	}
}

func TestTooltips(t *testing.T) {
	for state, want := range map[State]string{
		StateRunning:            "OmniSSHAgent - Running",
		StateDegraded:           "OmniSSHAgent - Degraded",
		StateConfigurationError: "OmniSSHAgent - Configuration error",
	} {
		if got := Tooltip(state); got != want {
			t.Fatalf("%s: %q", state, got)
		}
	}
}
