package repository

import (
	"context"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// ValveExecutionRepository owns all persistence operations for 阀门执行.
type ValveExecutionRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.ValveExecution], error)
	Get(context.Context, uint) (model.ValveExecution, error)
	Create(context.Context, *model.ValveExecution) error
	Update(context.Context, uint, uint, *model.ValveExecution) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type valveExecutionRepository struct {
	store *Store[model.ValveExecution]
}

func NewValveExecutionRepository(db *gorm.DB) ValveExecutionRepository {
	return &valveExecutionRepository{store: NewStore[model.ValveExecution](db)}
}

func (r *valveExecutionRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.ValveExecution], error) {
	return r.store.List(ctx, q)
}
func (r *valveExecutionRepository) Get(ctx context.Context, id uint) (model.ValveExecution, error) {
	return r.store.Get(ctx, id)
}
func (r *valveExecutionRepository) Create(ctx context.Context, item *model.ValveExecution) error {
	return r.store.Create(ctx, item)
}
func (r *valveExecutionRepository) Update(ctx context.Context, id, version uint, item *model.ValveExecution) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *valveExecutionRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *valveExecutionRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
