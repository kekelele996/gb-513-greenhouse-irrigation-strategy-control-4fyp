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

type IrrigationPlanService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.IrrigationPlan], error)
	Get(context.Context, uint) (model.IrrigationPlan, error)
	Create(context.Context, dto.CreateIrrigationPlan, string, string) (model.IrrigationPlan, error)
	Update(context.Context, uint, dto.UpdateIrrigationPlan, string, string) (model.IrrigationPlan, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.IrrigationPlan, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type irrigationPlanService struct {
	repository repository.IrrigationPlanRepository
	security   SecurityService
}

func NewIrrigationPlanService(repo repository.IrrigationPlanRepository, security SecurityService) IrrigationPlanService {
	return &irrigationPlanService{repository: repo, security: security}
}

func (s *irrigationPlanService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.IrrigationPlan], error) {
	return s.repository.List(ctx, query)
}

func (s *irrigationPlanService) Get(ctx context.Context, id uint) (model.IrrigationPlan, error) {
	return s.repository.Get(ctx, id)
}

func (s *irrigationPlanService) Create(ctx context.Context, input dto.CreateIrrigationPlan, actor, requestID string) (model.IrrigationPlan, error) {
	if err := validateIrrigationPlanBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.IrrigationPlan{}, err
	}
	item := model.IrrigationPlan{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.IrrigationPlanInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("create 灌溉计划: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "IrrigationPlan", item.ID, "", item.Status, "created 灌溉计划")
	return item, nil
}

func (s *irrigationPlanService) Update(ctx context.Context, id uint, input dto.UpdateIrrigationPlan, actor, requestID string) (model.IrrigationPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	if err := validateIrrigationPlanBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.IrrigationPlan{}, err
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
		return model.IrrigationPlan{}, fmt.Errorf("update 灌溉计划: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "IrrigationPlan", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *irrigationPlanService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.IrrigationPlan, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.IrrigationPlan{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.IrrigationPlanTransitions, current.Status, target) {
		return model.IrrigationPlan{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("transition 灌溉计划: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "IrrigationPlan", id, before, target, input.Reason); err != nil {
		return model.IrrigationPlan{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *irrigationPlanService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "IrrigationPlan", id, current.Status, "deleted", "soft deleted 灌溉计划")
}

func (s *irrigationPlanService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateIrrigationPlanBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
