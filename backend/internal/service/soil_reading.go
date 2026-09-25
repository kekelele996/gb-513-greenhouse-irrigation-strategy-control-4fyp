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

type SoilReadingService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.SoilReading], error)
	Get(context.Context, uint) (model.SoilReading, error)
	Create(context.Context, dto.CreateSoilReading, string, string) (model.SoilReading, error)
	Update(context.Context, uint, dto.UpdateSoilReading, string, string) (model.SoilReading, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.SoilReading, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type soilReadingService struct {
	repository repository.SoilReadingRepository
	security   SecurityService
}

func NewSoilReadingService(repo repository.SoilReadingRepository, security SecurityService) SoilReadingService {
	return &soilReadingService{repository: repo, security: security}
}

func (s *soilReadingService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.SoilReading], error) {
	return s.repository.List(ctx, query)
}

func (s *soilReadingService) Get(ctx context.Context, id uint) (model.SoilReading, error) {
	return s.repository.Get(ctx, id)
}

func (s *soilReadingService) Create(ctx context.Context, input dto.CreateSoilReading, actor, requestID string) (model.SoilReading, error) {
	if err := validateSoilReadingBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.SoilReading{}, err
	}
	item := model.SoilReading{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.SoilReadingInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
		ZoneCode:    strings.ToUpper(strings.TrimSpace(input.ZoneCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.SoilReading{}, fmt.Errorf("create 土壤读数: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "SoilReading", item.ID, "", item.Status, "created 土壤读数")
	return item, nil
}

func (s *soilReadingService) Update(ctx context.Context, id uint, input dto.UpdateSoilReading, actor, requestID string) (model.SoilReading, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.SoilReading{}, err
	}
	if err := validateSoilReadingBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.SoilReading{}, err
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
	current.ZoneCode = strings.ToUpper(strings.TrimSpace(input.ZoneCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.SoilReading{}, fmt.Errorf("update 土壤读数: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "SoilReading", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *soilReadingService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.SoilReading, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.SoilReading{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.SoilReadingTransitions, current.Status, target) {
		return model.SoilReading{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.SoilReading{}, fmt.Errorf("transition 土壤读数: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "SoilReading", id, before, target, input.Reason); err != nil {
		return model.SoilReading{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *soilReadingService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "SoilReading", id, current.Status, "deleted", "soft deleted 土壤读数")
}

func (s *soilReadingService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateSoilReadingBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
