package repository

import (
	"context"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// GreenhouseZoneRepository owns all persistence operations for 温室分区.
type GreenhouseZoneRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.GreenhouseZone], error)
	Get(context.Context, uint) (model.GreenhouseZone, error)
	Create(context.Context, *model.GreenhouseZone) error
	Update(context.Context, uint, uint, *model.GreenhouseZone) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type greenhouseZoneRepository struct {
	store *Store[model.GreenhouseZone]
}

func NewGreenhouseZoneRepository(db *gorm.DB) GreenhouseZoneRepository {
	return &greenhouseZoneRepository{store: NewStore[model.GreenhouseZone](db)}
}

func (r *greenhouseZoneRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.GreenhouseZone], error) {
	return r.store.List(ctx, q)
}
func (r *greenhouseZoneRepository) Get(ctx context.Context, id uint) (model.GreenhouseZone, error) {
	return r.store.Get(ctx, id)
}
func (r *greenhouseZoneRepository) Create(ctx context.Context, item *model.GreenhouseZone) error {
	return r.store.Create(ctx, item)
}
func (r *greenhouseZoneRepository) Update(ctx context.Context, id, version uint, item *model.GreenhouseZone) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *greenhouseZoneRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *greenhouseZoneRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
