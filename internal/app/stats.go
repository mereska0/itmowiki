package app

import "github.com/mereska0/itmowiki/internal/domain"

type StatsUseCase struct {
	repo domain.PageRepository
}

func NewStatsUseCase(repo domain.PageRepository) *StatsUseCase {
	return &StatsUseCase{repo: repo}
}

func (u *StatsUseCase) Execute() (domain.IndexStats, error) {
	return u.repo.Stats()
}
