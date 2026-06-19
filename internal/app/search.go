package app

import "github.com/mereska0/itmowiki/internal/domain"

type SearchUseCase struct {
	repo domain.PageRepository
}

func NewSearchUseCase(repo domain.PageRepository) *SearchUseCase {
	return &SearchUseCase{repo: repo}
}

func (u *SearchUseCase) Execute(query string) ([]domain.Page, error) {
	return u.repo.SearchPages(query)
}
