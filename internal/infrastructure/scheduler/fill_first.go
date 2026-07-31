package scheduler

import (
	"context"

	"cumulus/internal/domain"
)

type FillFirstScheduler struct{}

func NewFillFirstScheduler() *FillFirstScheduler {
	return &FillFirstScheduler{}
}

func (s *FillFirstScheduler) ChooseAccount(_ context.Context, request domain.ScheduleRequest) (*domain.GoogleAccount, error) {
	_ = request
	return nil, nil
}
