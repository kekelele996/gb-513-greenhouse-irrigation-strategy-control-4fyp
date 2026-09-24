package repository

import (
	"context"

	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/dto"
	"github.com/blueship581/greenhouse-irrigation-strategy-control/backend/internal/model"
	"gorm.io/gorm"
)

// SoilReadingRepository owns all persistence operations for 土壤读数.
type SoilReadingRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.SoilReading], error)
	Get(context.Context, uint) (model.SoilReading, error)
	Create(context.Context, *model.SoilReading) error
	Update(context.Context, uint, uint, *model.SoilReading) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type soilReadingRepository struct {
	store *Store[model.SoilReading]
}

func NewSoilReadingRepository(db *gorm.DB) SoilReadingRepository {
	return &soilReadingRepository{store: NewStore[model.SoilReading](db)}
}

func (r *soilReadingRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.SoilReading], error) {
	return r.store.List(ctx, q)
}
func (r *soilReadingRepository) Get(ctx context.Context, id uint) (model.SoilReading, error) {
	return r.store.Get(ctx, id)
}
func (r *soilReadingRepository) Create(ctx context.Context, item *model.SoilReading) error {
	return r.store.Create(ctx, item)
}
func (r *soilReadingRepository) Update(ctx context.Context, id, version uint, item *model.SoilReading) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *soilReadingRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *soilReadingRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
