package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/constants"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/repository"
)

type ValveExecutionService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ValveExecution], error)
	Get(context.Context, uint) (model.ValveExecution, error)
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
	security   SecurityService
}

func NewValveExecutionService(repo repository.ValveExecutionRepository, security SecurityService) ValveExecutionService {
	return &valveExecutionService{repository: repo, security: security}
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
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.ValveExecution{}, fmt.Errorf("create 阀门执行: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "ValveExecution", item.ID, "", item.Status, "created 阀门执行")
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
	before := current.Status
	current.Status = string(constants.ExecutionStateRunning)
	current.ControlConfirmedBy = actor
	current.ControlConfirmedAt = &now
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = now
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.ValveExecution{}, fmt.Errorf("confirm remote valve start: %w", err)
	}
	detail := fmt.Sprintf("requested by %s; confirmed independently: %s", current.ControlRequestedBy, input.Reason)
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
