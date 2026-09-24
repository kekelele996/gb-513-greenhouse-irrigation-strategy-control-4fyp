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

type GreenhouseZoneService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.GreenhouseZone], error)
	Get(context.Context, uint) (model.GreenhouseZone, error)
	Create(context.Context, dto.CreateGreenhouseZone, string, string) (model.GreenhouseZone, error)
	Update(context.Context, uint, dto.UpdateGreenhouseZone, string, string) (model.GreenhouseZone, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.GreenhouseZone, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type greenhouseZoneService struct {
	repository repository.GreenhouseZoneRepository
	security   SecurityService
}

func NewGreenhouseZoneService(repo repository.GreenhouseZoneRepository, security SecurityService) GreenhouseZoneService {
	return &greenhouseZoneService{repository: repo, security: security}
}

func (s *greenhouseZoneService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.GreenhouseZone], error) {
	return s.repository.List(ctx, query)
}

func (s *greenhouseZoneService) Get(ctx context.Context, id uint) (model.GreenhouseZone, error) {
	return s.repository.Get(ctx, id)
}

func (s *greenhouseZoneService) Create(ctx context.Context, input dto.CreateGreenhouseZone, actor, requestID string) (model.GreenhouseZone, error) {
	if err := validateGreenhouseZoneBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.GreenhouseZone{}, err
	}
	item := model.GreenhouseZone{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.GreenhouseZoneInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.GreenhouseZone{}, fmt.Errorf("create 温室分区: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "GreenhouseZone", item.ID, "", item.Status, "created 温室分区")
	return item, nil
}

func (s *greenhouseZoneService) Update(ctx context.Context, id uint, input dto.UpdateGreenhouseZone, actor, requestID string) (model.GreenhouseZone, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.GreenhouseZone{}, err
	}
	if err := validateGreenhouseZoneBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.GreenhouseZone{}, err
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
		return model.GreenhouseZone{}, fmt.Errorf("update 温室分区: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "GreenhouseZone", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *greenhouseZoneService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.GreenhouseZone, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.GreenhouseZone{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.GreenhouseZoneTransitions, current.Status, target) {
		return model.GreenhouseZone{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.GreenhouseZone{}, fmt.Errorf("transition 温室分区: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "GreenhouseZone", id, before, target, input.Reason); err != nil {
		return model.GreenhouseZone{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *greenhouseZoneService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "GreenhouseZone", id, current.Status, "deleted", "soft deleted 温室分区")
}

func (s *greenhouseZoneService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateGreenhouseZoneBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
