package model

import "time"

// IrrigationPlan models 灌溉计划 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type IrrigationPlan struct {
	BaseModel
	Facility    string    `json:"facility" gorm:"size:120;index"`
	Owner       string    `json:"owner" gorm:"size:120;index"`
	Category    string    `json:"category" gorm:"size:80;index"`
	RiskLevel   string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64   `json:"metricValue"`
	MetricUnit  string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Evidence    string    `json:"evidence" gorm:"size:2000"`
	RelatedCode string    `json:"relatedCode" gorm:"size:64;index"`
	// ZoneCode binds the plan to the zone it schedules irrigation for.
	ZoneCode string `json:"zoneCode" gorm:"size:64;index"`
	// StopMoisture is the plan's 停灌线 (volumetric water content, %): irrigation
	// must not start when the latest validated reading is at or above this line.
	StopMoisture float64 `json:"stopMoisture"`
}

func (item *IrrigationPlan) GetBase() *BaseModel { return &item.BaseModel }

func (item IrrigationPlan) TableName() string { return "irrigation_plans" }

var IrrigationPlanInitialStatus = "draft"
