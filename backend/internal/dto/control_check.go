package dto

import "time"

// PreStartCheckView is the read model shown to the independent reviewer before
// they confirm a remote valve start. It gathers every fact the review checks:
// running executions in the same zone, the latest validated soil reading and
// the linked plan's stop-moisture line. Conflicts explain why the start stays
// in 待启动 (planned) instead of entering the two-person start.
type PreStartCheckView struct {
	ExecutionID   uint   `json:"executionId"`
	ExecutionCode string `json:"executionCode"`
	ZoneCode      string `json:"zoneCode"`
	PlanCode      string `json:"planCode"`
	Status        string `json:"status"`
	// Result is "" when no check has run, otherwise passed/blocked.
	Result       string                `json:"result"`
	ConflictNo   string                `json:"conflictNo"`
	Detail       string                `json:"detail"`
	CheckedBy    string                `json:"checkedBy"`
	CheckedAt    *time.Time            `json:"checkedAt"`
	Zone         *PreStartZone         `json:"zone"`
	Plan         *PreStartPlan         `json:"plan"`
	Reading      *PreStartReading      `json:"reading"`
	ReadingFresh bool                  `json:"readingFresh"`
	RunningTasks []PreStartRunningTask `json:"runningTasks"`
	Conflicts    []PreStartConflict    `json:"conflicts"`
	Snapshot     string                `json:"snapshot"`
}

type PreStartZone struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type PreStartPlan struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	ZoneCode     string  `json:"zoneCode"`
	StopMoisture float64 `json:"stopMoisture"`
}

type PreStartReading struct {
	Code       string    `json:"code"`
	Status     string    `json:"status"`
	Moisture   float64   `json:"moisture"`
	MeasuredAt time.Time `json:"measuredAt"`
	ZoneCode   string    `json:"zoneCode"`
	AgeMinutes int64     `json:"ageMinutes"`
}

type PreStartRunningTask struct {
	ID             uint      `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	ZoneCode       string    `json:"zoneCode"`
	Status         string    `json:"status"`
	StartedRequest string    `json:"startedRequest"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type PreStartConflict struct {
	Code        string     `json:"code"`
	Kind        string     `json:"kind"`
	Message     string     `json:"message"`
	ReadingTime *time.Time `json:"readingTime,omitempty"`
	Moisture    *float64   `json:"moisture,omitempty"`
}
