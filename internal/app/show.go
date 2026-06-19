package app

import "github.com/mereska0/itmowiki/internal/domain"

type ShowUseCase struct {
	repo domain.PageRepository
}

func NewShowUseCase(repo domain.PageRepository) *ShowUseCase {
	return &ShowUseCase{repo: repo}
}

func (s *ShowUseCase) Execute(id int) (domain.Page, error) {
	return s.repo.GetPageByID(id)
}
