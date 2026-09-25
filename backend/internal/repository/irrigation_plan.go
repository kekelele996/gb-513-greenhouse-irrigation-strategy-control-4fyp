package repository

import (
	"context"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// IrrigationPlanRepository owns all persistence operations for 灌溉计划.
type IrrigationPlanRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.IrrigationPlan], error)
	Get(context.Context, uint) (model.IrrigationPlan, error)
	GetByCode(context.Context, string) (model.IrrigationPlan, error)
	Create(context.Context, *model.IrrigationPlan) error
	Update(context.Context, uint, uint, *model.IrrigationPlan) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type irrigationPlanRepository struct {
	store *Store[model.IrrigationPlan]
}

func NewIrrigationPlanRepository(db *gorm.DB) IrrigationPlanRepository {
	return &irrigationPlanRepository{store: NewStore[model.IrrigationPlan](db)}
}

func (r *irrigationPlanRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.IrrigationPlan], error) {
	return r.store.List(ctx, q)
}
func (r *irrigationPlanRepository) Get(ctx context.Context, id uint) (model.IrrigationPlan, error) {
	return r.store.Get(ctx, id)
}
func (r *irrigationPlanRepository) GetByCode(ctx context.Context, code string) (model.IrrigationPlan, error) {
	var item model.IrrigationPlan
	err := r.store.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	return item, err
}
func (r *irrigationPlanRepository) Create(ctx context.Context, item *model.IrrigationPlan) error {
	return r.store.Create(ctx, item)
}
func (r *irrigationPlanRepository) Update(ctx context.Context, id, version uint, item *model.IrrigationPlan) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *irrigationPlanRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *irrigationPlanRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
