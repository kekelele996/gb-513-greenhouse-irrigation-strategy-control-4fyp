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
	// ZoneCode identifies the greenhouse zone governed by this strategy version.
	ZoneCode string `json:"zoneCode" gorm:"size:64;index"`
	// StopMoistureLine is the soil moisture percentage at which irrigation must
	// stop. A validated reading at or above this line blocks a remote start.
	StopMoistureLine float64 `json:"stopMoistureLine"`
}

func (item *IrrigationPlan) GetBase() *BaseModel { return &item.BaseModel }

func (item IrrigationPlan) TableName() string { return "irrigation_plans" }

var IrrigationPlanInitialStatus = "draft"
