package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/constants"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"gorm.io/gorm"
)

// preStartChecker evaluates the evidence a reviewer must inspect before a
// remote valve start: running tasks in the same zone, the latest validated
// soil reading and the plan stop-moisture line.
type preStartChecker struct {
	zones    repository.GreenhouseZoneRepository
	plans    repository.IrrigationPlanRepository
	readings repository.SoilReadingRepository
	execs    repository.ValveExecutionRepository
}

func newPreStartChecker(zones repository.GreenhouseZoneRepository, plans repository.IrrigationPlanRepository, readings repository.SoilReadingRepository, execs repository.ValveExecutionRepository) *preStartChecker {
	return &preStartChecker{zones: zones, plans: plans, readings: readings, execs: execs}
}

// evaluate always returns a fully populated snapshot. Conflicts carries every
// blocking problem found; an empty slice means the check passed.
func (c *preStartChecker) evaluate(ctx context.Context, execution model.ValveExecution, reviewer string, checkedAt time.Time) *model.ControlCheckSnapshot {
	snapshot := &model.ControlCheckSnapshot{
		CheckedAt:          checkedAt,
		CheckedBy:          reviewer,
		Status:             model.ControlCheckStatusPassed,
		ZoneCode:           execution.ZoneCode,
		PlanCode:           execution.PlanCode,
		RunningTasks:       make([]model.ControlRunningTask, 0),
		Conflicts:          make([]model.ControlConflictEvidence, 0),
		FreshnessWindowMin: int(constants.PreStartReadingFreshness / time.Minute),
	}

	zone, zoneErr := c.zones.FindByCode(ctx, execution.ZoneCode)
	if zoneErr != nil {
		if errors.Is(zoneErr, gorm.ErrRecordNotFound) {
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictZoneMissing, ZoneCode: execution.ZoneCode,
				Message: fmt.Sprintf("分区 %s 不存在，无法核对同分区运行任务与读数", execution.ZoneCode),
			})
		}
	} else {
		snapshot.ZoneName = zone.Name
	}

	plan, planErr := c.plans.FindByCode(ctx, execution.PlanCode)
	if planErr != nil {
		if errors.Is(planErr, gorm.ErrRecordNotFound) {
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictPlanMissing, PlanCode: execution.PlanCode,
				Message: fmt.Sprintf("灌溉计划 %s 不存在，无法核对计划停灌线", execution.PlanCode),
			})
		}
	} else {
		snapshot.PlanName = plan.Name
		snapshot.PlanStatus = plan.Status
		snapshot.StopMoistureLine = plan.StopMoistureLine
		if zoneErr == nil && plan.ZoneCode != "" && !strings.EqualFold(plan.ZoneCode, execution.ZoneCode) {
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictPlanZoneMismatch, ZoneCode: execution.ZoneCode, PlanCode: execution.PlanCode,
				Message: fmt.Sprintf("计划 %s 归属分区 %s，与执行分区 %s 不一致", execution.PlanCode, plan.ZoneCode, execution.ZoneCode),
			})
		}
	}

	running, err := c.execs.RunningByZone(ctx, execution.ZoneCode, execution.ID)
	if err == nil {
		for _, task := range running {
			entry := model.ControlRunningTask{ID: task.ID, Code: task.Code, Name: task.Name, Status: task.Status, ControlConfirmedBy: task.ControlConfirmedBy}
			if task.ControlConfirmedAt != nil {
				entry.StartedAt = *task.ControlConfirmedAt
			}
			snapshot.RunningTasks = append(snapshot.RunningTasks, entry)
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictRunningTask, ZoneCode: execution.ZoneCode, ExecutionID: task.ID,
				Message: fmt.Sprintf("同分区任务 %s 正在运行（%s），该分区可能已经浇水", task.Code, task.Name),
			})
		}
	}

	reading, readingErr := c.readings.LatestValidatedByZone(ctx, execution.ZoneCode)
	switch {
	case errors.Is(readingErr, gorm.ErrRecordNotFound):
		snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
			Kind: model.ControlConflictReadingMissing, ZoneCode: execution.ZoneCode,
			Message: fmt.Sprintf("分区 %s 没有已校验（validated）土壤读数，不能凭未校验读数启动", execution.ZoneCode),
		})
	case readingErr == nil:
		age := checkedAt.Sub(reading.EffectiveAt.UTC())
		evidence := &model.ControlReadingEvidence{
			ID: reading.ID, Code: reading.Code, Moisture: reading.MetricValue,
			MeasuredAt: reading.EffectiveAt.UTC(), Status: reading.Status,
			AgeMinutes: int(age / time.Minute),
		}
		snapshot.LatestReading = evidence
		if age > constants.PreStartReadingFreshness {
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictReadingStale, ZoneCode: execution.ZoneCode,
				ReadingCode: reading.Code, ReadingTime: reading.EffectiveAt.UTC(), Moisture: reading.MetricValue,
				Message: fmt.Sprintf("最近已校验读数 %s 采集于 %s（%.0f 分钟前），超过 %d 分钟有效期，禁止拿旧读数启动",
					reading.Code, reading.EffectiveAt.UTC().Format(time.RFC3339), age.Minutes(), snapshot.FreshnessWindowMin),
			})
		}
		if planErr == nil && reading.MetricValue >= plan.StopMoistureLine {
			snapshot.Conflicts = append(snapshot.Conflicts, model.ControlConflictEvidence{
				Kind: model.ControlConflictAboveStopMoisture, ZoneCode: execution.ZoneCode, PlanCode: execution.PlanCode,
				ReadingCode: reading.Code, ReadingTime: reading.EffectiveAt.UTC(), Moisture: reading.MetricValue,
				StopLine: plan.StopMoistureLine,
				Message: fmt.Sprintf("最近已校验读数 %s 含水率 %.1f%% 已达到/超过计划停灌线 %.1f%%，无需浇水",
					reading.Code, reading.MetricValue, plan.StopMoistureLine),
			})
		}
	}

	if len(snapshot.Conflicts) > 0 {
		snapshot.Status = model.ControlCheckStatusBlocked
		numberConflicts(snapshot)
	}
	return snapshot
}

// numberConflicts assigns a shared batch number (CF-<timestamp>-<seq>) and
// per-conflict suffixes so reviewers can reference exact conflicts.
func numberConflicts(snapshot *model.ControlCheckSnapshot) {
	batch := fmt.Sprintf("CF-%s-%04d", snapshot.CheckedAt.UTC().Format("20060102T150405Z"), len(snapshot.Conflicts))
	for i := range snapshot.Conflicts {
		snapshot.Conflicts[i].Code = fmt.Sprintf("%s-%d", batch, i+1)
	}
}

// marshalSnapshot persists the frozen check evidence as JSON.
func marshalSnapshot(snapshot *model.ControlCheckSnapshot) (string, error) {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal control check snapshot: %w", err)
	}
	return string(raw), nil
}

// describeConflicts renders the human-readable 控制详情 text containing every
// conflict number, reading time and moisture value.
func describeConflicts(snapshot *model.ControlCheckSnapshot) string {
	lines := make([]string, 0, len(snapshot.Conflicts)+1)
	lines = append(lines, fmt.Sprintf("启动前复核未通过（%s），状态保持待启动，冲突 %d 项：", snapshot.CheckedAt.UTC().Format("2006-01-02 15:04:05 UTC"), len(snapshot.Conflicts)))
	for _, conflict := range snapshot.Conflicts {
		line := fmt.Sprintf("[%s] %s", conflict.Code, conflict.Message)
		if !conflict.ReadingTime.IsZero() {
			line += fmt.Sprintf("（读数时间 %s，含水率 %.1f%%）", conflict.ReadingTime.UTC().Format(time.RFC3339), conflict.Moisture)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// describePass renders the 控制详情 text for a successful check.
func describePass(snapshot *model.ControlCheckSnapshot) string {
	reading := "无已校验读数"
	if snapshot.LatestReading != nil {
		reading = fmt.Sprintf("读数 %s 含水率 %.1f%%（%s，%d 分钟前）",
			snapshot.LatestReading.Code, snapshot.LatestReading.Moisture,
			snapshot.LatestReading.MeasuredAt.UTC().Format(time.RFC3339), snapshot.LatestReading.AgeMinutes)
	}
	return fmt.Sprintf("启动前复核通过（%s）：同分区运行任务 %d 个；%s；计划 %s 停灌线 %.1f%%。",
		snapshot.CheckedAt.UTC().Format("2006-01-02 15:04:05 UTC"), len(snapshot.RunningTasks), reading,
		snapshot.PlanCode, snapshot.StopMoistureLine)
}
