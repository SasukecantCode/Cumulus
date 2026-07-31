package usecase

import (
	"context"

	"cumulus/internal/domain"
)

type SchedulerService struct {
	scheduler domain.Scheduler
}

func NewSchedulerService(scheduler domain.Scheduler) *SchedulerService {
	return &SchedulerService{scheduler: scheduler}
}

func (s *SchedulerService) ChooseAccount(ctx context.Context, request domain.ScheduleRequest) (*domain.GoogleAccount, error) {
	return s.scheduler.ChooseAccount(ctx, request)
}
