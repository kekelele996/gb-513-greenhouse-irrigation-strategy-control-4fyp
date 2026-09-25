package dto

import "github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"

// ControlDetailResponse is the 控制详情 payload: the execution aggregate plus
// the live or frozen pre-start check evidence the reviewer must inspect.
type ControlDetailResponse struct {
	Execution model.ValveExecution        `json:"execution"`
	Snapshot  *model.ControlCheckSnapshot `json:"snapshot"`
	// Live indicates whether Snapshot was just evaluated against current data
	// (control-detail preview) rather than restored from the persisted snapshot.
	Live bool `json:"live"`
}
