package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

type Notifier interface {
	NotifyRenewal(ctx context.Context, when time.Time) error
}

type Scheduler struct {
	cron *cron.Cron
}

func New() *Scheduler {
	return &Scheduler{cron: cron.New()}
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) ScheduleDailyNotifications(n Notifier) error {
	_, err := s.cron.AddFunc("0 9 * * *", func() {
		ctx := context.Background()
		if err := n.NotifyRenewal(ctx, time.Now()); err != nil {
			log.Printf("notify renewal: %v", err)
		}
	})
	return err
}
