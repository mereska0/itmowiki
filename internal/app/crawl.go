package app

import (
	service "github.com/mereska0/itmowiki/internal/crawler"
	"github.com/mereska0/itmowiki/internal/domain"
)

type CrawlUseCase struct {
	crawler *service.Service
}

type CrawlProgress struct {
	Current int
	Total   int
	URL     string
	Cached  bool
}

func NewCrawlUseCase(repo domain.PageRepository) *CrawlUseCase {
	return &CrawlUseCase{
		crawler: service.NewService(repo),
	}
}

func (u *CrawlUseCase) Execute(
	startURL string,
	limit int,
	onProgress func(CrawlProgress),
) (int, error) {
	pages, err := u.crawler.CrawlWithProgress(startURL, limit, func(p service.Progress) {
		if onProgress != nil {
			onProgress(CrawlProgress{
				Current: p.Current,
				Total:   p.Total,
				URL:     p.URL,
				Cached:  p.Cached,
			})
		}
	})
	if err != nil {
		return 0, err
	}

	return len(pages), nil
}
