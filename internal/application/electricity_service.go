package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gamee1910/volt/internal/common/utils"
	"github.com/gamee1910/volt/internal/domain/entity"
	"github.com/gamee1910/volt/internal/domain/repository"
	"github.com/gamee1910/volt/pkg/evnhcmc"
	"github.com/gamee1910/volt/pkg/logger"
)

type ElectricityService struct {
	repo      repository.ElectricityRepository
	evnClient evnhcmc.EVNClient
	log       *logger.Logger
	now       func() time.Time
}

func NewElectricityService(
	repo repository.ElectricityRepository,
	evnClient evnhcmc.EVNClient,
	log *logger.Logger,
) *ElectricityService {
	return &ElectricityService{
		repo:      repo,
		evnClient: evnClient,
		log:       log,
		now:       time.Now,
	}
}

func (s *ElectricityService) Sync(ctx context.Context, fromDate, toDate string) (err error) {
	start := time.Now()
	s.log.Info("sync electricity: started", "from_date", fromDate, "to_date", toDate)
	defer func() {
		durationMs := time.Since(start).Milliseconds()
		if err != nil {
			s.log.Error("sync electricity: failed", "duration_ms", durationMs, "error", err)
			return
		}
		s.log.Info("sync electricity: done", "duration_ms", durationMs)
	}()

	resp, err := s.evnClient.GetDailyPowerUsageData(ctx, evnhcmc.DailyPowerUsageRequest{
		FromDate: fromDate,
		ToDate:   toDate,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch EVN data: %w", err)
	}

	location, err := utils.LoadVietnamTimezone()
	if err != nil {
		return err
	}

	for _, item := range resp.Data.DailyOutputs {
		output := strings.TrimSpace(item.TotalOutput)
		// Tong also carries daily usage; use it when the display value is unavailable.
		unavailable := output == "!"
		if unavailable && (item.TotalIndex <= 0 || math.IsNaN(item.TotalIndex) || math.IsInf(item.TotalIndex, 0)) {
			s.log.Warnw("sync electricity: skipping unavailable daily output", "date", item.FullDate, "total_output", item.TotalOutput)
			continue
		}
		kwh := item.TotalIndex
		if !unavailable {
			kwh, err = strconv.ParseFloat(output, 64)
			if err != nil {
				return fmt.Errorf("invalid total output %q at date %s: %w", item.TotalOutput, item.FullDate, err)
			}
		} else {
			s.log.Info("sync electricity: using numeric daily output", "date", item.FullDate, "kwh", kwh)
		}
		if kwh == 0 {
			kwh = item.TotalIndex
		}

		measurementDate, err := utils.ParseEVNDate(item.FullDate, location)
		if err != nil {
			return fmt.Errorf("failed to parse evn date %q: %w", item.FullDate, err)
		}

		consumption := &entity.ElectricityConsumption{
			MeasurementDate: measurementDate,
			ConsumptionKWh:  kwh,
		}
		if err := s.repo.Upsert(ctx, consumption); err != nil {
			return fmt.Errorf("failed to upsert consumption at date %q: %w", item.FullDate, err)
		}
	}
	return nil
}

func (s *ElectricityService) SyncCurrentMonth(ctx context.Context) error {
	fromDate, toDate, err := s.CurrentMonthRange()
	if err != nil {
		return err
	}
	return s.Sync(ctx, fromDate, toDate)
}

func (s *ElectricityService) CurrentMonthRange() (fromDate, toDate string, err error) {
	location, err := utils.LoadVietnamTimezone()
	if err != nil {
		return "", "", err
	}
	now := s.now().In(location)
	return utils.FormatEVNDate(utils.GetFirstDayOfMonth(now, location)), utils.FormatEVNDate(now), nil
}

func (s *ElectricityService) Yesterday(ctx context.Context) (ElectricityConsumption, error) {
	location, err := utils.LoadVietnamTimezone()
	if err != nil {
		return ElectricityConsumption{}, err
	}

	now := s.now().In(location)
	yesterday := utils.GetYesterday(now, location)
	firstDayOfMonth := utils.GetFirstDayOfMonth(now, location)

	consumption, err := s.repo.FetchByDate(ctx, yesterday)
	if err != nil {
		if errors.Is(err, repository.ErrConsumptionNotFound) {
			return ElectricityConsumption{}, &UsageUnavailableError{Date: yesterday}
		}
		return ElectricityConsumption{}, fmt.Errorf("failed to fetch yesterday usage: %w", err)
	}

	monthKWh, err := s.repo.CalculateTotalConsumptionFromDateToDate(ctx, firstDayOfMonth, yesterday)
	if err != nil {
		return ElectricityConsumption{}, fmt.Errorf("failed to calculate total consumption: %w", err)
	}

	return ElectricityConsumption{
		MeasurementDate: consumption.MeasurementDate,
		ConsumptionKWh:  consumption.ConsumptionKWh,
		TotalAmount:     utils.CalculateElectricityBill(monthKWh),
	}, nil
}

func (s *ElectricityService) All(ctx context.Context) (ElectricitySummary, error) {
	rows, err := s.repo.FetchAll(ctx)
	if err != nil {
		return ElectricitySummary{}, fmt.Errorf("failed to fetch all electricity: %w", err)
	}

	var (
		totalKWh float64
		data     []*ElectricityConsumption
	)
	for _, row := range rows {
		data = append(data, &ElectricityConsumption{
			MeasurementDate: row.MeasurementDate,
			ConsumptionKWh:  row.ConsumptionKWh,
			TotalAmount:     utils.CalculateElectricityBill(row.ConsumptionKWh),
		})
		totalKWh += row.ConsumptionKWh
	}

	return ElectricitySummary{
		TotalKWh:    totalKWh,
		TotalAmount: utils.CalculateElectricityBill(totalKWh),
		Data:        data,
	}, nil
}
