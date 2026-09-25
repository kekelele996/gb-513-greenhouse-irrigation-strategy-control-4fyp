package service

import (
	"context"
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
	ControlDetail(context.Context, uint) (dto.ControlDetailResponse, error)
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
	plans      repository.IrrigationPlanRepository
	readings   repository.SoilReadingRepository
	security   SecurityService
	checker    *preStartChecker
}

func NewValveExecutionService(repo repository.ValveExecutionRepository, zones repository.GreenhouseZoneRepository, plans repository.IrrigationPlanRepository, readings repository.SoilReadingRepository, security SecurityService) ValveExecutionService {
	return &valveExecutionService{
		repository: repo, zones: zones, plans: plans, readings: readings, security: security,
		checker: newPreStartChecker(zones, plans, readings, repo),
	}
}

func (s *valveExecutionService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ValveExecution], error) {
	return s.repository.List(ctx, query)
}

func (s *valveExecutionService) Get(ctx context.Context, id uint) (model.ValveExecution, error) {
	return s.repository.Get(ctx, id)
}

// ControlDetail returns the execution with the persisted check snapshot when
// one exists; otherwise it evaluates current data as a live preview so the
// reviewer sees running tasks, the latest validated reading and the stop line
// before confirming.
func (s *valveExecutionService) ControlDetail(ctx context.Context, id uint) (dto.ControlDetailResponse, error) {
	execution, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.ControlDetailResponse{}, err
	}
	snapshot, err := execution.Snapshot()
	if err != nil {
		return dto.ControlDetailResponse{}, fmt.Errorf("decode control check snapshot: %w", err)
	}
	if snapshot != nil {
		return dto.ControlDetailResponse{Execution: execution, Snapshot: snapshot, Live: false}, nil
	}
	live := s.checker.evaluate(ctx, execution, "", time.Now().UTC())
	return dto.ControlDetailResponse{Execution: execution, Snapshot: live, Live: true}, nil
}

func (s *valveExecutionService) Create(ctx context.Context, input dto.CreateValveExecution, actor, requestID string) (model.ValveExecution, error) {
	if err := validateValveExecutionBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ValveExecution{}, err
	}
	zoneCode := strings.ToUpper(strings.TrimSpace(input.ZoneCode))
	planCode := strings.ToUpper(strings.TrimSpace(input.PlanCode))
	if err := s.validateReferences(ctx, zoneCode, planCode); err != nil {
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
		ControlCheckStatus: model.ControlCheckStatusPending,
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.ValveExecution{}, fmt.Errorf("create 阀门执行: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "ValveExecution", item.ID, "", item.Status, fmt.Sprintf("created 阀门执行 linked to zone %s plan %s", zoneCode, planCode))
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
	if (current.ControlRequestedBy != "" || current.Status != string(constants.ExecutionStatePlanned)) &&
		(zoneCode != current.ZoneCode || planCode != current.PlanCode) {
		return model.ValveExecution{}, ErrReferenceLocked
	}
	if err := s.validateReferences(ctx, zoneCode, planCode); err != nil {
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

// validateReferences ensures the execution points at an existing zone and
// irrigation plan whose ZoneCode matches the execution zone.
func (s *valveExecutionService) validateReferences(ctx context.Context, zoneCode, planCode string) error {
	if zoneCode == "" || planCode == "" {
		return ErrInvalidInput
	}
	if _, err := s.zones.FindByCode(ctx, zoneCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: zone %s does not exist", ErrInvalidInput, zoneCode)
		}
		return err
	}
	plan, err := s.plans.FindByCode(ctx, planCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: plan %s does not exist", ErrInvalidInput, planCode)
		}
		return err
	}
	if plan.ZoneCode != "" && !strings.EqualFold(plan.ZoneCode, zoneCode) {
		return fmt.Errorf("%w: plan %s targets zone %s, not %s", ErrInvalidInput, planCode, plan.ZoneCode, zoneCode)
	}
	return nil
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
	snapshot := s.checker.evaluate(ctx, current, actor, now)
	rawSnapshot, err := marshalSnapshot(snapshot)
	if err != nil {
		return model.ValveExecution{}, err
	}
	current.ControlCheckSnapshot = rawSnapshot

	if snapshot.Status == model.ControlCheckStatusBlocked {
		// Conflicts freeze the execution in planned. The reviewer must resolve
		// them (stop the other task, refresh the reading, etc.) and re-confirm.
		batchNo := ""
		if len(snapshot.Conflicts) > 0 {
			batchNo = snapshot.Conflicts[0].Code
			if idx := strings.LastIndex(batchNo, "-"); idx > 3 {
				batchNo = batchNo[:idx]
			}
		}
		current.ControlCheckStatus = model.ControlCheckStatusBlocked
		current.ControlConflictNo = batchNo
		current.ControlDetail = describeConflicts(snapshot)
		current.Version = input.ExpectedVersion + 1
		current.UpdatedAt = now
		if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
			return model.ValveExecution{}, fmt.Errorf("persist blocked control check: %w", err)
		}
		if err := s.security.Audit(ctx, actor, requestID, "control_conflict", "ValveExecution", id, current.Status, current.Status, current.ControlDetail); err != nil {
			return model.ValveExecution{}, fmt.Errorf("persist control conflict audit: %w", err)
		}
		return s.repository.Get(ctx, id)
	}

	before := current.Status
	current.Status = string(constants.ExecutionStateRunning)
	current.ControlConfirmedBy = actor
	current.ControlConfirmedAt = &now
	current.ControlCheckStatus = model.ControlCheckStatusPassed
	current.ControlConflictNo = ""
	current.ControlDetail = describePass(snapshot)
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("confirm remote valve start: %w", err)
	}
	detail := fmt.Sprintf("requested by %s; confirmed independently: %s | %s", current.ControlRequestedBy, input.Reason, current.ControlDetail)
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
