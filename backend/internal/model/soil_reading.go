package model

import "time"

// SoilReading models 土壤读数 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type SoilReading struct {
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
	// ZoneCode links the reading to the greenhouse zone it was measured in.
	// Pre-start checks use the latest validated reading of the execution's zone.
	ZoneCode string `json:"zoneCode" gorm:"size:64;index"`
}

func (item *SoilReading) GetBase() *BaseModel { return &item.BaseModel }

func (item SoilReading) TableName() string { return "soil_readings" }

var SoilReadingInitialStatus = "fresh"
