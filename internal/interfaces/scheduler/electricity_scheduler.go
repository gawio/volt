package scheduler

import (
	"context"
	"fmt"

	"github.com/gamee1910/volt/internal/application"
	"github.com/gamee1910/volt/internal/common/utils"
	"github.com/gamee1910/volt/pkg/logger"
	"github.com/robfig/cron/v3"
)

const (
	every12HoursCron = "0 */12 * * *"
)

type SyncNotifier interface {
	NotifySyncResult(ctx context.Context, syncErr error) error
}

type ElectricityScheduler struct {
	cron     *cron.Cron
	service  *application.ElectricityService
	log      *logger.Logger
	notifier SyncNotifier
}

func NewElectricityScheduler(
	service *application.ElectricityService,
	log *logger.Logger,
	notifier SyncNotifier,
) (*ElectricityScheduler, error) {
	location, err := utils.LoadVietnamTimezone()
	if err != nil {
		return nil, fmt.Errorf("failed to load location: %w", err)
	}
	option := cron.WithLocation(location)
	c := cron.New(option)

	s := &ElectricityScheduler{
		cron:     c,
		service:  service,
		log:      log,
		notifier: notifier,
	}

	_, err = c.AddFunc(every12HoursCron, s.run)
	if err != nil {
		return nil, fmt.Errorf("register electricity scheduler cron error: %w", err)
	}

	return s, nil
}

func (s *ElectricityScheduler) run() {
	ctx := context.Background()
	syncErr := s.sync(ctx)
	if syncErr != nil {
		s.log.Error("electricity_sync_job: failed", "error", syncErr.Error())
	} else {
		s.log.Info("electricity_sync_job: success")
	}
	s.notify(ctx, syncErr)
}

func (s *ElectricityScheduler) sync(ctx context.Context) error {
	return s.service.SyncCurrentMonth(ctx)
}

func (s *ElectricityScheduler) notify(ctx context.Context, syncErr error) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.NotifySyncResult(ctx, syncErr); err != nil {
		s.log.Error("electricity_sync_job: notify failed", "error", err.Error())
	}
}

func (s *ElectricityScheduler) Start() {
	s.log.Infof("electricity_scheduler: started, schedule=%s", every12HoursCron)
	go s.run()
	s.cron.Start()
}

func (s *ElectricityScheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.log.Info("electricity_scheduler: stopped")
}
