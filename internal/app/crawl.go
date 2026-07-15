package app

import (
	service "github.com/mereska0/itmowiki/internal/crawler"
	"github.com/mereska0/itmowiki/internal/domain"
)

const DefaultCrawlWorkers = 8

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
	return u.ExecuteWithWorkers(startURL, limit, DefaultCrawlWorkers, onProgress)
}

func (u *CrawlUseCase) ExecuteWithWorkers(
	startURL string,
	limit int,
	workers int,
	onProgress func(CrawlProgress),
) (int, error) {
	pages, err := u.crawler.CrawlWithWorkers(startURL, limit, workers, func(p service.Progress) {
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
