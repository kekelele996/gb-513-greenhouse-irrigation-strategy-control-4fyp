package constants

import "time"

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

// ReadingFreshnessWindow bounds how old the latest validated soil reading may
// be at remote valve start: night shifts must not confirm against stale data.
const ReadingFreshnessWindow = 30 * time.Minute

type ZoneState string

const (
	ZoneStateActive ZoneState = "active"
	ZoneStateDry    ZoneState = "dry"
	ZoneStateWet    ZoneState = "wet"
	ZoneStateLocked ZoneState = "locked"
)

var AllZoneState = []string{"active", "dry", "wet", "locked"}

type ExecutionState string

const (
	ExecutionStatePlanned   ExecutionState = "planned"
	ExecutionStateRunning   ExecutionState = "running"
	ExecutionStateSucceeded ExecutionState = "succeeded"
	ExecutionStateFailed    ExecutionState = "failed"
)

var AllExecutionState = []string{"planned", "running", "succeeded", "failed"}

var GreenhouseZoneTransitions = map[string]map[string]bool{
	"active": {"dry": true, "wet": true},
	"dry":    {"wet": true, "locked": true, "active": true},
	"wet":    {"locked": true, "dry": true},
	"locked": {"wet": true},
}

var SoilReadingTransitions = map[string]map[string]bool{
	"fresh":     {"validated": true, "anomalous": true},
	"validated": {"anomalous": true, "expired": true, "fresh": true},
	"anomalous": {"expired": true, "validated": true},
	"expired":   {"anomalous": true},
}

var IrrigationPlanTransitions = map[string]map[string]bool{
	"draft":     {"approved": true, "scheduled": true},
	"approved":  {"scheduled": true, "completed": true, "draft": true},
	"scheduled": {"completed": true, "approved": true},
	"completed": {"scheduled": true},
}

var ValveExecutionTransitions = map[string]map[string]bool{
	"planned":   {"running": true},
	"running":   {"succeeded": true, "failed": true, "planned": true},
	"succeeded": {"failed": true, "running": true},
	"failed":    {"succeeded": true},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
