package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gamee1910/volt/internal/domain/entity"
)

var ErrConsumptionNotFound = errors.New("electricity consumption not found")

type ElectricityRepository interface {
	Upsert(ctx context.Context, req *entity.ElectricityConsumption) error
	FetchByDate(ctx context.Context, date time.Time) (*entity.ElectricityConsumption, error)
	FetchAll(ctx context.Context) ([]*entity.ElectricityConsumption, error)
	CalculateTotalConsumptionFromDateToDate(ctx context.Context, fromDate, toDate time.Time) (float64, error)
}
