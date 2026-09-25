package model

import (
	"encoding/json"
	"time"
)

// Control check outcomes. A failed check never moves the execution out of
// planned; the reviewer must resolve every conflict and confirm again.
const (
	ControlCheckStatusPending = "pending"
	ControlCheckStatusPassed  = "passed"
	ControlCheckStatusBlocked = "blocked"
)

// Conflict kinds raised while a reviewer confirms a remote valve start.
const (
	ControlConflictZoneMissing       = "zone_missing"
	ControlConflictPlanMissing       = "plan_missing"
	ControlConflictRunningTask       = "running_task_same_zone"
	ControlConflictReadingMissing    = "validated_reading_missing"
	ControlConflictReadingStale      = "validated_reading_stale"
	ControlConflictAboveStopMoisture = "above_stop_moisture_line"
	ControlConflictPlanZoneMismatch  = "plan_zone_mismatch"
)

// ControlConflictEvidence describes a single blocking conflict. ReadingTime
// and Moisture are populated whenever the conflict is backed by a soil reading.
type ControlConflictEvidence struct {
	Code        string    `json:"code"`
	Kind        string    `json:"kind"`
	Message     string    `json:"message"`
	ReadingCode string    `json:"readingCode,omitempty"`
	ReadingTime time.Time `json:"readingTime,omitempty"`
	Moisture    float64   `json:"moisture,omitempty"`
	ExecutionID uint      `json:"executionId,omitempty"`
	ZoneCode    string    `json:"zoneCode,omitempty"`
	PlanCode    string    `json:"planCode,omitempty"`
	StopLine    float64   `json:"stopLine,omitempty"`
}

// ControlCheckSnapshot freezes the evidence the reviewer actually saw at
// confirmation time: running tasks in the same zone, the latest validated
// reading and the plan stop-moisture line. It is persisted verbatim so later
// audits can reconstruct why a start was blocked or allowed.
type ControlCheckSnapshot struct {
	CheckedAt          time.Time                 `json:"checkedAt"`
	CheckedBy          string                    `json:"checkedBy"`
	Status             string                    `json:"status"`
	ZoneCode           string                    `json:"zoneCode"`
	ZoneName           string                    `json:"zoneName,omitempty"`
	PlanCode           string                    `json:"planCode"`
	PlanName           string                    `json:"planName,omitempty"`
	PlanStatus         string                    `json:"planStatus,omitempty"`
	StopMoistureLine   float64                   `json:"stopMoistureLine,omitempty"`
	RunningTasks       []ControlRunningTask      `json:"runningTasks"`
	LatestReading      *ControlReadingEvidence   `json:"latestReading,omitempty"`
	FreshnessWindowMin int                       `json:"freshnessWindowMinutes"`
	Conflicts          []ControlConflictEvidence `json:"conflicts"`
}

// ControlRunningTask is another valve execution already running in the same
// zone. Watering the zone twice concurrently is exactly what night-shift
// reviews must catch.
type ControlRunningTask struct {
	ID                 uint      `json:"id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	Status             string    `json:"status"`
	StartedAt          time.Time `json:"startedAt,omitempty"`
	ControlConfirmedBy string    `json:"controlConfirmedBy,omitempty"`
}

// ControlReadingEvidence captures the most recent validated soil reading for
// the linked zone.
type ControlReadingEvidence struct {
	ID         uint      `json:"id"`
	Code       string    `json:"code"`
	Moisture   float64   `json:"moisture"`
	MeasuredAt time.Time `json:"measuredAt"`
	Status     string    `json:"status"`
	AgeMinutes int       `json:"ageMinutes"`
}

// ValveExecution models 阀门执行 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type ValveExecution struct {
	BaseModel
	Facility           string     `json:"facility" gorm:"size:120;index"`
	Owner              string     `json:"owner" gorm:"size:120;index"`
	Category           string     `json:"category" gorm:"size:80;index"`
	RiskLevel          string     `json:"riskLevel" gorm:"size:32;index"`
	MetricValue        float64    `json:"metricValue"`
	MetricUnit         string     `json:"metricUnit" gorm:"size:24"`
	EffectiveAt        time.Time  `json:"effectiveAt"`
	Evidence           string     `json:"evidence" gorm:"size:2000"`
	RelatedCode        string     `json:"relatedCode" gorm:"size:64;index"`
	ControlRequestedBy string     `json:"controlRequestedBy" gorm:"size:80;index"`
	ControlRequestedAt *time.Time `json:"controlRequestedAt"`
	ControlConfirmedBy string     `json:"controlConfirmedBy" gorm:"size:80;index"`
	ControlConfirmedAt *time.Time `json:"controlConfirmedAt"`
	// ZoneCode and PlanCode bind the execution record to the greenhouse zone it
	// waters and the irrigation plan version it executes.
	ZoneCode string `json:"zoneCode" gorm:"size:64;index"`
	PlanCode string `json:"planCode" gorm:"size:64;index"`
	// ControlCheckStatus records the last pre-start review: pending, passed or
	// blocked. Blocked leaves the execution in planned.
	ControlCheckStatus string `json:"controlCheckStatus" gorm:"size:24;index"`
	// ControlConflictNo identifies the last blocked check batch, e.g.
	// CF-20260924T031205Z-0001. Individual conflicts append -1/-2 suffixes.
	ControlConflictNo string `json:"controlConflictNo" gorm:"size:48;index"`
	// ControlDetail carries the human-readable conflict summary (reading time,
	// moisture and conflict numbers) rendered in 控制详情.
	ControlDetail string `json:"controlDetail" gorm:"size:2000"`
	// ControlCheckSnapshot stores the frozen JSON snapshot from the last check.
	ControlCheckSnapshot string `json:"controlCheckSnapshot" gorm:"type:text"`
}

// Snapshot decodes the persisted pre-start check snapshot.
func (item *ValveExecution) Snapshot() (*ControlCheckSnapshot, error) {
	if item.ControlCheckSnapshot == "" {
		return nil, nil
	}
	var snapshot ControlCheckSnapshot
	if err := json.Unmarshal([]byte(item.ControlCheckSnapshot), &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (item *ValveExecution) GetBase() *BaseModel { return &item.BaseModel }

func (item ValveExecution) TableName() string { return "valve_executions" }

var ValveExecutionInitialStatus = "planned"
