package constants

import "testing"

func TestGreenhouseZoneTransitionGraph(t *testing.T) {
	if !CanTransition(GreenhouseZoneTransitions, "active", "dry") {
		t.Fatalf("expected active -> dry transition to be allowed")
	}
	if CanTransition(GreenhouseZoneTransitions, "active", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}
