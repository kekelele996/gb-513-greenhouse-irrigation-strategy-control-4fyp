package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/constants"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
	"gorm.io/gorm"
)

type ValveExecutionService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ValveExecution], error)
	Get(context.Context, uint) (model.ValveExecution, error)
	PreStartCheck(context.Context, uint) (dto.PreStartCheckView, error)
	Create(context.Context, dto.CreateValveExecution, string, string) (model.ValveExecution, error)
	Update(context.Context, uint, dto.UpdateValveExecution, string, string) (model.ValveExecution, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.ValveExecution, error)
	RequestControl(context.Context, uint, dto.ControlConfirmationRequest, string, string) (model.ValveExecution, error)
	ConfirmControl(context.Context, uint, dto.ControlConfirmationRequest, string, string) (model.ValveExecution, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type valveExecutionService struct {
	repository repository.ValveExecutionRepository
	zones      repository.GreenhouseZoneRepository
	readings   repository.SoilReadingRepository
	plans      repository.IrrigationPlanRepository
	security   SecurityService
}

func NewValveExecutionService(
	repo repository.ValveExecutionRepository,
	zones repository.GreenhouseZoneRepository,
	readings repository.SoilReadingRepository,
	plans repository.IrrigationPlanRepository,
	security SecurityService,
) ValveExecutionService {
	return &valveExecutionService{repository: repo, zones: zones, readings: readings, plans: plans, security: security}
}

// conflictKind labels the pre-start review failures so the UI and audit trail
// share the same vocabulary.
const (
	conflictZoneMissing    = "ZONE_NOT_FOUND"
	conflictPlanMissing    = "PLAN_NOT_FOUND"
	conflictPlanZone       = "PLAN_ZONE_MISMATCH"
	conflictPlanNoStopLine = "PLAN_MISSING_STOP_LINE"
	conflictRunningTask    = "RUNNING_TASK_IN_ZONE"
	conflictNoReading      = "NO_VALIDATED_READING"
	conflictStaleReading   = "STALE_READING"
	conflictMoistureHigh   = "MOISTURE_ABOVE_STOP_LINE"
)

type controlConflict struct {
	kind    string
	message string
	reading *model.SoilReading
}

func (s *valveExecutionService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ValveExecution], error) {
	return s.repository.List(ctx, query)
}

func (s *valveExecutionService) Get(ctx context.Context, id uint) (model.ValveExecution, error) {
	return s.repository.Get(ctx, id)
}

func (s *valveExecutionService) Create(ctx context.Context, input dto.CreateValveExecution, actor, requestID string) (model.ValveExecution, error) {
	if err := validateValveExecutionBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ValveExecution{}, err
	}
	zoneCode := strings.ToUpper(strings.TrimSpace(input.ZoneCode))
	planCode := strings.ToUpper(strings.TrimSpace(input.PlanCode))
	if err := s.validateLinks(ctx, zoneCode, planCode); err != nil {
		return model.ValveExecution{}, err
	}
	item := model.ValveExecution{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ValveExecutionInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
		ZoneCode:    zoneCode, PlanCode: planCode,
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.ValveExecution{}, fmt.Errorf("create 阀门执行: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "ValveExecution", item.ID, "", item.Status,
		fmt.Sprintf("created 阀门执行 linked to zone %s and plan %s", zoneCode, planCode))
	return item, nil
}

func (s *valveExecutionService) Update(ctx context.Context, id uint, input dto.UpdateValveExecution, actor, requestID string) (model.ValveExecution, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ValveExecution{}, err
	}
	if err := validateValveExecutionBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ValveExecution{}, err
	}
	zoneCode := strings.ToUpper(strings.TrimSpace(input.ZoneCode))
	planCode := strings.ToUpper(strings.TrimSpace(input.PlanCode))
	if err := s.validateLinks(ctx, zoneCode, planCode); err != nil {
		return model.ValveExecution{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.ZoneCode = zoneCode
	current.PlanCode = planCode
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("update 阀门执行: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "ValveExecution", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *valveExecutionService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.ValveExecution, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ValveExecution{}, err
	}
	target := strings.TrimSpace(input.Status)
	if current.Status == string(constants.ExecutionStatePlanned) && target == string(constants.ExecutionStateRunning) {
		return model.ValveExecution{}, ErrDualConfirmation
	}
	if !constants.CanTransition(constants.ValveExecutionTransitions, current.Status, target) {
		return model.ValveExecution{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("transition 阀门执行: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "ValveExecution", id, before, target, input.Reason); err != nil {
		return model.ValveExecution{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *valveExecutionService) RequestControl(ctx context.Context, id uint, input dto.ControlConfirmationRequest, actor, requestID string) (model.ValveExecution, error) {
	if !input.Confirmed {
		return model.ValveExecution{}, ErrExplicitConfirmation
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ValveExecution{}, err
	}
	if current.Status != string(constants.ExecutionStatePlanned) {
		return model.ValveExecution{}, fmt.Errorf("%w: only planned executions can request remote start", ErrInvalidTransition)
	}
	if current.ControlRequestedBy != "" {
		return model.ValveExecution{}, ErrControlRequested
	}
	now := time.Now().UTC()
	current.ControlRequestedBy = actor
	current.ControlRequestedAt = &now
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("request remote valve start: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "control_request", "ValveExecution", id, current.Status, current.Status, input.Reason); err != nil {
		return model.ValveExecution{}, fmt.Errorf("persist control request audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

// PreStartCheck gathers the reviewer's decision material without mutating the
// execution: running tasks in the same zone, the latest validated reading and
// its age, and the plan's stop-moisture line.
func (s *valveExecutionService) PreStartCheck(ctx context.Context, id uint) (dto.PreStartCheckView, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.PreStartCheckView{}, err
	}
	view := s.buildCheckView(ctx, current, time.Now().UTC())
	return view, nil
}

func (s *valveExecutionService) ConfirmControl(ctx context.Context, id uint, input dto.ControlConfirmationRequest, actor, requestID string) (model.ValveExecution, error) {
	if !input.Confirmed {
		return model.ValveExecution{}, ErrExplicitConfirmation
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ValveExecution{}, err
	}
	if current.Status != string(constants.ExecutionStatePlanned) {
		return model.ValveExecution{}, fmt.Errorf("%w: only planned executions can be remotely started", ErrInvalidTransition)
	}
	if current.ControlRequestedBy == "" || current.ControlRequestedAt == nil {
		return model.ValveExecution{}, ErrControlNotRequested
	}
	if current.ControlRequestedBy == actor {
		return model.ValveExecution{}, ErrSelfConfirmation
	}
	if !constants.CanTransition(constants.ValveExecutionTransitions, current.Status, string(constants.ExecutionStateRunning)) {
		return model.ValveExecution{}, ErrInvalidTransition
	}

	now := time.Now().UTC()
	view := s.buildCheckView(ctx, current, now)

	// Conflicts keep the execution in 待启动: record reading time, moisture and
	// the conflict number in the control detail, but never start the valve.
	if len(view.Conflicts) > 0 {
		nextSeq := nextConflictSequence(current.ControlConflictNo)
		detail := buildBlockedDetail(view, nextSeq)
		current.ControlCheckResult = model.ControlCheckBlocked
		current.ControlCheckBy = actor
		current.ControlCheckAt = &now
		current.ControlConflictNo = fmt.Sprintf("CFL-%s-%03d", current.Code, nextSeq)
		current.ControlDetail = detail
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = now
		if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
			return model.ValveExecution{}, fmt.Errorf("persist blocked control check: %w", err)
		}
		if err := s.security.Audit(ctx, actor, requestID, "control_blocked", "ValveExecution", id, current.Status, current.Status, detail); err != nil {
			return model.ValveExecution{}, fmt.Errorf("persist blocked control audit: %w", err)
		}
		return s.repository.Get(ctx, id)
	}

	snapshot := buildPassedSnapshot(view, actor, now)
	before := current.Status
	current.Status = string(constants.ExecutionStateRunning)
	current.ControlConfirmedBy = actor
	current.ControlConfirmedAt = &now
	current.ControlCheckResult = model.ControlCheckPassed
	current.ControlCheckBy = actor
	current.ControlCheckAt = &now
	current.ControlConflictNo = ""
	current.ControlDetail = fmt.Sprintf("启动前复核通过：同分区运行中任务 %d 个；最近已校验读数 %s；%s。%s",
		len(view.RunningTasks), readingLabel(view.Reading), stopLineLabel(view.Plan), input.Reason)
	current.ControlCheckSnapshot = snapshot
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("confirm remote valve start: %w", err)
	}
	detail := fmt.Sprintf("requested by %s; pre-start check passed with saved snapshot; confirmed independently: %s",
		current.ControlRequestedBy, input.Reason)
	if err := s.security.Audit(ctx, actor, requestID, "control_confirm", "ValveExecution", id, before, current.Status, detail); err != nil {
		return model.ValveExecution{}, fmt.Errorf("persist control confirmation audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *valveExecutionService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "ValveExecution", id, current.Status, "deleted", "soft deleted 阀门执行")
}

func (s *valveExecutionService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateValveExecutionBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}

// validateLinks enforces at write time that the execution points at a real zone
// and at a plan scheduled for that same zone.
func (s *valveExecutionService) validateLinks(ctx context.Context, zoneCode, planCode string) error {
	if zoneCode == "" || planCode == "" {
		return ErrInvalidInput
	}
	zone, err := s.zones.GetByCode(ctx, zoneCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrZoneNotFound, zoneCode)
		}
		return err
	}
	plan, err := s.plans.GetByCode(ctx, planCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrPlanNotFound, planCode)
		}
		return err
	}
	if !planCoversZone(plan, zone) {
		return fmt.Errorf("%w: plan %s is not scheduled for zone %s", ErrPlanZoneMismatch, planCode, zoneCode)
	}
	return nil
}

func planCoversZone(plan model.IrrigationPlan, zone model.GreenhouseZone) bool {
	if strings.EqualFold(strings.TrimSpace(plan.ZoneCode), zone.Code) {
		return true
	}
	// Legacy records created before explicit zone links shared RelatedCode.
	return strings.TrimSpace(plan.ZoneCode) == "" && strings.TrimSpace(plan.RelatedCode) != "" &&
		strings.EqualFold(strings.TrimSpace(plan.RelatedCode), strings.TrimSpace(zone.RelatedCode))
}

type checkMaterial struct {
	zone         *model.GreenhouseZone
	plan         *model.IrrigationPlan
	reading      *model.SoilReading
	readingFound bool
	running      []model.ValveExecution
	conflicts    []controlConflict
}

// buildCheckView collects every fact the independent review needs and derives
// the conflict list. It performs no writes so the reviewer can preview it.
func (s *valveExecutionService) buildCheckView(ctx context.Context, execution model.ValveExecution, now time.Time) dto.PreStartCheckView {
	view := dto.PreStartCheckView{
		ExecutionID:   execution.ID,
		ExecutionCode: execution.Code,
		ZoneCode:      execution.ZoneCode,
		PlanCode:      execution.PlanCode,
		Status:        execution.Status,
		Result:        execution.ControlCheckResult,
		ConflictNo:    execution.ControlConflictNo,
		Detail:        execution.ControlDetail,
		CheckedBy:     execution.ControlCheckBy,
		CheckedAt:     execution.ControlCheckAt,
		Snapshot:      execution.ControlCheckSnapshot,
		RunningTasks:  []dto.PreStartRunningTask{},
		Conflicts:     []dto.PreStartConflict{},
	}
	material := s.loadCheckMaterial(ctx, execution, now)

	if material.zone != nil {
		view.Zone = &dto.PreStartZone{Code: material.zone.Code, Name: material.zone.Name, Status: material.zone.Status}
	}
	if material.plan != nil {
		view.Plan = &dto.PreStartPlan{
			Code: material.plan.Code, Name: material.plan.Name, Status: material.plan.Status,
			ZoneCode: linkedZoneCode(material.plan.ZoneCode, material.zone), StopMoisture: material.plan.StopMoisture,
		}
	}
	for _, task := range material.running {
		view.RunningTasks = append(view.RunningTasks, dto.PreStartRunningTask{
			ID: task.ID, Code: task.Code, Name: task.Name, ZoneCode: task.ZoneCode,
			Status: task.Status, StartedRequest: task.ControlRequestedBy, UpdatedAt: task.UpdatedAt,
		})
	}
	if material.readingFound && material.reading != nil {
		age := now.Sub(material.reading.EffectiveAt)
		view.ReadingFresh = age >= 0 && age <= constants.ReadingFreshnessWindow
		view.Reading = &dto.PreStartReading{
			Code: material.reading.Code, Status: material.reading.Status,
			Moisture: material.reading.MetricValue, MeasuredAt: material.reading.EffectiveAt,
			ZoneCode: linkedZoneCode(material.reading.ZoneCode, material.zone), AgeMinutes: int64(age.Minutes()),
		}
	}
	for _, conflict := range material.conflicts {
		item := dto.PreStartConflict{Code: conflictKindLabel(conflict.kind), Kind: conflict.kind, Message: conflict.message}
		if conflict.reading != nil {
			t := conflict.reading.EffectiveAt
			item.ReadingTime = &t
			item.Moisture = &conflict.reading.MetricValue
		}
		view.Conflicts = append(view.Conflicts, item)
	}
	return view
}

func (s *valveExecutionService) loadCheckMaterial(ctx context.Context, execution model.ValveExecution, now time.Time) checkMaterial {
	material := checkMaterial{conflicts: []controlConflict{}}

	zone, err := s.zones.GetByCode(ctx, execution.ZoneCode)
	if err != nil {
		material.conflicts = append(material.conflicts, controlConflict{
			kind:    conflictZoneMissing,
			message: fmt.Sprintf("执行记录关联的分区 %s 不存在，无法核对同分区运行情况", execution.ZoneCode),
		})
	} else {
		material.zone = &zone
	}

	plan, err := s.plans.GetByCode(ctx, execution.PlanCode)
	if err != nil {
		material.conflicts = append(material.conflicts, controlConflict{
			kind:    conflictPlanMissing,
			message: fmt.Sprintf("执行记录关联的灌溉计划 %s 不存在，无法读取计划停灌线", execution.PlanCode),
		})
	} else {
		material.plan = &plan
		if material.zone != nil && !planCoversZone(plan, zone) {
			material.conflicts = append(material.conflicts, controlConflict{
				kind:    conflictPlanZone,
				message: fmt.Sprintf("灌溉计划 %s 不属于分区 %s，计划与执行分区不一致", plan.Code, zone.Code),
			})
		}
		if plan.StopMoisture <= 0 {
			material.conflicts = append(material.conflicts, controlConflict{
				kind:    conflictPlanNoStopLine,
				message: fmt.Sprintf("灌溉计划 %s 尚未设置停灌线，无法判定是否应停灌", plan.Code),
			})
		}
	}

	running, err := s.repository.RunningByZone(ctx, execution.ZoneCode, execution.ID)
	if err == nil {
		material.running = running
	}
	for _, task := range running {
		material.conflicts = append(material.conflicts, controlConflict{
			kind: conflictRunningTask,
			message: fmt.Sprintf("同分区已有运行中任务 %s（%s），该分区可能正在浇水；请求人 %s",
				task.Code, task.Name, fallbackName(task.ControlRequestedBy, "未知")),
		})
	}

	if material.zone != nil {
		reading, err := s.readings.LatestValidatedByZone(ctx, zone.Code, zone.RelatedCode)
		switch {
		case err == nil:
			material.reading = &reading
			material.readingFound = true
			age := now.Sub(reading.EffectiveAt)
			if age > constants.ReadingFreshnessWindow {
				material.conflicts = append(material.conflicts, controlConflict{
					kind: conflictStaleReading, reading: &reading,
					message: fmt.Sprintf("最近一次已校验读数时间 %s，含水率 %.1f%%，距今 %d 分钟，超过 %.0f 分钟有效期，不得凭旧读数启动",
						formatReadingTime(reading.EffectiveAt), reading.MetricValue, int64(age.Minutes()), constants.ReadingFreshnessWindow.Minutes()),
				})
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			material.conflicts = append(material.conflicts, controlConflict{
				kind:    conflictNoReading,
				message: fmt.Sprintf("分区 %s 没有状态为已校验（validated）的土壤读数，缺少启动依据", zone.Code),
			})
		}
	}

	if material.plan != nil && material.reading != nil && material.plan.StopMoisture > 0 {
		age := now.Sub(material.reading.EffectiveAt)
		if age >= 0 && age <= constants.ReadingFreshnessWindow && material.reading.MetricValue >= material.plan.StopMoisture {
			material.conflicts = append(material.conflicts, controlConflict{
				kind:    conflictMoistureHigh,
				reading: material.reading,
				message: readingMoistureConflictMessage(material.reading, material.plan),
			})
		}
	}

	return material
}

func readingMoistureConflictMessage(reading *model.SoilReading, plan *model.IrrigationPlan) string {
	return fmt.Sprintf("最近一次已校验读数时间 %s，含水率 %.1f%%，已达到/超过计划 %s 的停灌线 %.1f%%，按计划应停止浇水",
		formatReadingTime(reading.EffectiveAt), reading.MetricValue, plan.Code, plan.StopMoisture)
}

// nextConflictSequence continues the per-execution conflict numbering so a
// blocked review can be distinguished from the next attempt.
func nextConflictSequence(previous string) int {
	base := strings.TrimSpace(previous)
	if base == "" {
		return 1
	}
	parts := strings.Split(base, "-")
	var seq int
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &seq); err == nil && seq > 0 {
		return seq + 1
	}
	return 1
}

func buildBlockedDetail(view dto.PreStartCheckView, seq int) string {
	base := fmt.Sprintf("CFL-%s-%03d", view.ExecutionCode, seq)
	lines := []string{fmt.Sprintf("启动前复核未通过，状态保持待启动（planned）。冲突编号 %s，共 %d 项冲突：", base, len(view.Conflicts))}
	for idx, conflict := range view.Conflicts {
		code := fmt.Sprintf("%s-%c", base, 'A'+idx)
		reading := ""
		if conflict.ReadingTime != nil {
			reading = fmt.Sprintf("；读数时间 %s", formatReadingTime(*conflict.ReadingTime))
		}
		if conflict.Moisture != nil {
			reading += fmt.Sprintf("；含水率 %.1f%%", *conflict.Moisture)
		}
		lines = append(lines, fmt.Sprintf("[%s %s]%s %s", code, conflict.Code, reading, conflict.Message))
	}
	detail := strings.Join(lines, " ")
	if len(detail) > 1900 {
		detail = detail[:1900] + "…"
	}
	return detail
}

type controlSnapshot struct {
	CheckedAt      string  `json:"checkedAt"`
	CheckedBy      string  `json:"checkedBy"`
	ZoneCode       string  `json:"zoneCode"`
	PlanCode       string  `json:"planCode"`
	PlanStatus     string  `json:"planStatus"`
	StopMoisture   float64 `json:"stopMoisture"`
	ReadingCode    string  `json:"readingCode"`
	ReadingStatus  string  `json:"readingStatus"`
	ReadingTime    string  `json:"readingTime"`
	Moisture       float64 `json:"moisture"`
	ReadingAgeMins int64   `json:"readingAgeMinutes"`
	RunningTasks   int     `json:"runningTasksInZone"`
	Result         string  `json:"result"`
}

func buildPassedSnapshot(view dto.PreStartCheckView, reviewer string, now time.Time) string {
	snapshot := controlSnapshot{
		CheckedAt: now.Format(time.RFC3339), CheckedBy: reviewer,
		ZoneCode: view.ZoneCode, PlanCode: view.PlanCode, Result: model.ControlCheckPassed,
		RunningTasks: len(view.RunningTasks),
	}
	if view.Plan != nil {
		snapshot.PlanStatus = view.Plan.Status
		snapshot.StopMoisture = view.Plan.StopMoisture
	}
	if view.Reading != nil {
		snapshot.ReadingCode = view.Reading.Code
		snapshot.ReadingStatus = view.Reading.Status
		snapshot.ReadingTime = view.Reading.MeasuredAt.Format(time.RFC3339)
		snapshot.Moisture = view.Reading.Moisture
		snapshot.ReadingAgeMins = view.Reading.AgeMinutes
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Sprintf(`{"result":"%s","checkedBy":%q}`, model.ControlCheckPassed, reviewer)
	}
	return string(encoded)
}

func conflictKindLabel(kind string) string {
	switch kind {
	case conflictZoneMissing:
		return "分区不存在"
	case conflictPlanMissing:
		return "计划不存在"
	case conflictPlanZone:
		return "计划分区不一致"
	case conflictPlanNoStopLine:
		return "计划缺少停灌线"
	case conflictRunningTask:
		return "同分区运行中任务"
	case conflictNoReading:
		return "缺少已校验读数"
	case conflictStaleReading:
		return "读数已过期"
	case conflictMoistureHigh:
		return "含水率达到停灌线"
	default:
		return kind
	}
}

func formatReadingTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 MST")
}

func readingLabel(reading *dto.PreStartReading) string {
	if reading == nil {
		return "无"
	}
	return fmt.Sprintf("%s（%s，%.1f%%，%d 分钟前）", reading.Code, formatReadingTime(reading.MeasuredAt), reading.Moisture, reading.AgeMinutes)
}

func stopLineLabel(plan *dto.PreStartPlan) string {
	if plan == nil {
		return "计划缺失"
	}
	return fmt.Sprintf("计划 %s 停灌线 %.1f%%", plan.Code, plan.StopMoisture)
}

func linkedZoneCode(code string, zone *model.GreenhouseZone) string {
	if strings.TrimSpace(code) != "" {
		return code
	}
	if zone != nil {
		return zone.Code
	}
	return ""
}

func fallbackName(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
