package application

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/gamee1910/volt/internal/common/utils"
	"github.com/gamee1910/volt/internal/domain/entity"
	"github.com/gamee1910/volt/internal/domain/repository"
	"github.com/gamee1910/volt/pkg/evnhcmc"
	"github.com/gamee1910/volt/pkg/logger"
)

type fakeRepo struct {
	rows map[string]*entity.ElectricityConsumption
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: map[string]*entity.ElectricityConsumption{}}
}

func key(t time.Time) string { return t.Format("2006-01-02") }

func (r *fakeRepo) Upsert(_ context.Context, c *entity.ElectricityConsumption) error {
	r.rows[key(c.MeasurementDate)] = c
	return nil
}

func (r *fakeRepo) FetchByDate(_ context.Context, date time.Time) (*entity.ElectricityConsumption, error) {
	c, ok := r.rows[key(date)]
	if !ok {
		return nil, repository.ErrConsumptionNotFound
	}
	return c, nil
}

func (r *fakeRepo) FetchAll(_ context.Context) ([]*entity.ElectricityConsumption, error) {
	var out []*entity.ElectricityConsumption
	for _, c := range r.rows {
		out = append(out, c)
	}
	return out, nil
}

func (r *fakeRepo) CalculateTotalConsumptionFromDateToDate(_ context.Context, from, to time.Time) (float64, error) {
	var total float64
	for _, c := range r.rows {
		if !c.MeasurementDate.Before(from) && !c.MeasurementDate.After(to) {
			total += c.ConsumptionKWh
		}
	}
	return total, nil
}

type fakeEVN struct {
	outputs []evnhcmc.DailyPowerUsage
	got     evnhcmc.DailyPowerUsageRequest
}

func (f *fakeEVN) GetDailyPowerUsageData(_ context.Context, req evnhcmc.DailyPowerUsageRequest) (*evnhcmc.DailyPowerUsageResponse, error) {
	f.got = req
	return &evnhcmc.DailyPowerUsageResponse{Data: evnhcmc.DailyPowerUsageData{DailyOutputs: f.outputs}}, nil
}

func newTestService(t *testing.T, repo *fakeRepo, evn *fakeEVN, now time.Time) *ElectricityService {
	t.Helper()
	s := NewElectricityService(repo, evn, logger.NewLogger("test"))
	s.now = func() time.Time { return now }
	return s
}

func vietnamTime(t *testing.T, year int, month time.Month, day, hour int) time.Time {
	t.Helper()
	loc, err := utils.LoadVietnamTimezone()
	if err != nil {
		t.Fatal(err)
	}
	return time.Date(year, month, day, hour, 0, 0, 0, loc)
}

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestYesterday_MissingUsageReturnsDatedUnavailableError(t *testing.T) {
	s := newTestService(t, newFakeRepo(), &fakeEVN{}, vietnamTime(t, 2026, time.November, 8, 0))
	_, err := s.Yesterday(context.Background())
	var unavailable *UsageUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected UsageUnavailableError, got %v", err)
	}
	if key(unavailable.Date) != "2026-11-07" {
		t.Fatalf("missing date = %s, want 2026-11-07", key(unavailable.Date))
	}
	if len(s.repo.(*fakeRepo).rows) != 0 {
		t.Fatal("missing usage must not create a zero consumption record")
	}
}

func TestSync_UpsertsEachDayAndFallsBackToTotalIndex(t *testing.T) {
	repo := newFakeRepo()
	evn := &fakeEVN{outputs: []evnhcmc.DailyPowerUsage{
		{FullDate: "01/11/2026", TotalOutput: "12.5"},
		{FullDate: "02/11/2026", TotalOutput: "0", TotalIndex: 9.75},
	}}
	s := newTestService(t, repo, evn, time.Now())

	if err := s.Sync(context.Background(), "01/11/2026", "02/11/2026"); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if evn.got.FromDate != "01/11/2026" || evn.got.ToDate != "02/11/2026" {
		t.Errorf("EVN called with %+v", evn.got)
	}
	if got := repo.rows["2026-11-01"].ConsumptionKWh; got != 12.5 {
		t.Errorf("day 1 kWh = %v, want 12.5", got)
	}
	if got := repo.rows["2026-11-02"].ConsumptionKWh; got != 9.75 {
		t.Errorf("day 2 kWh = %v, want 9.75 (TotalIndex fallback)", got)
	}
}

func TestSync_InvalidOutputFails(t *testing.T) {
	evn := &fakeEVN{outputs: []evnhcmc.DailyPowerUsage{{FullDate: "01/11/2026", TotalOutput: "abc"}}}
	s := newTestService(t, newFakeRepo(), evn, time.Now())

	if err := s.Sync(context.Background(), "01/11/2026", "01/11/2026"); err == nil {
		t.Fatal("expected error for invalid TotalOutput")
	}
}

func TestSync_UnavailableOutputFallsBackToNumericDailyUsage(t *testing.T) {
	for _, output := range []string{"!", " ! "} {
		t.Run(output, func(t *testing.T) {
			repo := newFakeRepo()
			evn := &fakeEVN{outputs: []evnhcmc.DailyPowerUsage{
				{FullDate: "07/11/2026", TotalOutput: output, TotalIndex: 12.53},
			}}
			s := newTestService(t, repo, evn, vietnamTime(t, 2026, time.November, 8, 9))
			if err := s.SyncCurrentMonth(context.Background()); err != nil {
				t.Fatalf("SyncCurrentMonth: %v", err)
			}
			got, err := s.Yesterday(context.Background())
			if err != nil {
				t.Fatalf("Yesterday: %v", err)
			}
			if got.ConsumptionKWh != 12.53 || key(got.MeasurementDate) != "2026-11-07" {
				t.Fatalf("Yesterday = %+v, want 07/11/2026 with 12.53 kWh", got)
			}
		})
	}
}

func TestSync_SkipsUnavailableOutputAndRetriesLater(t *testing.T) {
	for _, output := range []string{"!", " ! "} {
		t.Run(output, func(t *testing.T) {
			repo := newFakeRepo()
			evn := &fakeEVN{outputs: []evnhcmc.DailyPowerUsage{
				{FullDate: "06/11/2026", TotalOutput: "12.5"},
				{FullDate: "07/11/2026", TotalOutput: output},
				{FullDate: "08/11/2026", TotalOutput: "9.75"},
			}}
			s := newTestService(t, repo, evn, time.Now())
			sync := func() {
				t.Helper()
				if err := s.Sync(context.Background(), "06/11/2026", "08/11/2026"); err != nil {
					t.Fatalf("Sync: %v", err)
				}
			}
			sync()
			if len(repo.rows) != 2 || repo.rows["2026-11-06"].ConsumptionKWh != 12.5 || repo.rows["2026-11-08"].ConsumptionKWh != 9.75 {
				t.Fatalf("expected only the two valid days to be saved: %+v", repo.rows)
			}
			evn.outputs[1].TotalOutput = "10.25"
			sync()
			if got := repo.rows["2026-11-07"].ConsumptionKWh; got != 10.25 {
				t.Fatalf("retried day kWh = %v, want 10.25", got)
			}
			evn.outputs[1].TotalOutput = output
			sync()
			if got := repo.rows["2026-11-07"].ConsumptionKWh; got != 10.25 {
				t.Fatalf("unavailable output overwrote existing usage: %v", got)
			}
		})
	}
}

func TestSyncCurrentMonth_UsesFirstOfMonthToToday(t *testing.T) {
	evn := &fakeEVN{}
	s := newTestService(t, newFakeRepo(), evn, vietnamTime(t, 2026, time.November, 4, 10))

	if err := s.SyncCurrentMonth(context.Background()); err != nil {
		t.Fatalf("SyncCurrentMonth: %v", err)
	}
	if evn.got.FromDate != "01/11/2026" || evn.got.ToDate != "04/11/2026" {
		t.Errorf("EVN called with %+v, want 01/11/2026 -> 04/11/2026", evn.got)
	}
}

func TestYesterday_ReturnsDayUsageAndMonthToDateBill(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["2026-10-31"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.October, 31, 0), ConsumptionKWh: 100}
	repo.rows["2026-11-01"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.November, 1, 0), ConsumptionKWh: 20}
	repo.rows["2026-11-02"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.November, 2, 0), ConsumptionKWh: 40}
	repo.rows["2026-11-03"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.November, 3, 0), ConsumptionKWh: 10}
	s := newTestService(t, repo, &fakeEVN{}, vietnamTime(t, 2026, time.November, 3, 9))

	got, err := s.Yesterday(context.Background())
	if err != nil {
		t.Fatalf("Yesterday: %v", err)
	}

	if key(got.MeasurementDate) != "2026-11-02" || got.ConsumptionKWh != 40 {
		t.Errorf("got %s %v kWh, want 2026-11-02 40 kWh", key(got.MeasurementDate), got.ConsumptionKWh)
	}
	if want := utils.CalculateElectricityBill(60); !almostEqual(got.TotalAmount, want) {
		t.Errorf("TotalAmount = %v, want %v", got.TotalAmount, want)
	}
}

func TestAll_BillsEachDayAndTheTotal(t *testing.T) {
	repo := newFakeRepo()
	repo.rows["2026-11-01"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.November, 1, 0), ConsumptionKWh: 30}
	repo.rows["2026-11-02"] = &entity.ElectricityConsumption{MeasurementDate: vietnamTime(t, 2026, time.November, 2, 0), ConsumptionKWh: 40}
	s := newTestService(t, repo, &fakeEVN{}, time.Now())

	got, err := s.All(context.Background())
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	if len(got.Data) != 2 || got.TotalKWh != 70 {
		t.Fatalf("got %d rows, %v kWh; want 2 rows, 70 kWh", len(got.Data), got.TotalKWh)
	}
	if want := utils.CalculateElectricityBill(70); !almostEqual(got.TotalAmount, want) {
		t.Errorf("TotalAmount = %v, want %v", got.TotalAmount, want)
	}
	for _, d := range got.Data {
		if want := utils.CalculateElectricityBill(d.ConsumptionKWh); !almostEqual(d.TotalAmount, want) {
			t.Errorf("day %s TotalAmount = %v, want %v", key(d.MeasurementDate), d.TotalAmount, want)
		}
	}
}
