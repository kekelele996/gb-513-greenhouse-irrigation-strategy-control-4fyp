package model

import "time"

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

	// ZoneCode/PlanCode attach the execution to its greenhouse zone and the
	// irrigation plan being executed; pre-start checks are scoped to that zone.
	ZoneCode string `json:"zoneCode" gorm:"size:64;index"`
	PlanCode string `json:"planCode" gorm:"size:64;index"`

	// ControlCheckResult records the latest independent pre-start review:
	// pass enters the two-person start, blocked keeps the execution planned.
	ControlCheckResult   string     `json:"controlCheckResult" gorm:"size:24;index"`
	ControlCheckBy       string     `json:"controlCheckBy" gorm:"size:80;index"`
	ControlCheckAt       *time.Time `json:"controlCheckAt"`
	ControlConflictNo    string     `json:"controlConflictNo" gorm:"size:64;index"`
	ControlDetail        string     `json:"controlDetail" gorm:"size:2000"`
	ControlCheckSnapshot string     `json:"controlCheckSnapshot" gorm:"type:text"`
}

func (item *ValveExecution) GetBase() *BaseModel { return &item.BaseModel }

func (item ValveExecution) TableName() string { return "valve_executions" }

var ValveExecutionInitialStatus = "planned"

// Pre-start review outcomes stored in ControlCheckResult.
const (
	ControlCheckPassed  = "passed"
	ControlCheckBlocked = "blocked"
)
