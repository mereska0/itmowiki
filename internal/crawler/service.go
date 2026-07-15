package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mereska0/itmowiki/internal/domain"
	parser "github.com/mereska0/itmowiki/internal/parser/html"
)

type Service struct {
	client http.Client
	repo   domain.PageRepository
}

type Progress struct {
	Current int
	Total   int
	URL     string
	Cached  bool
}

type crawlJob struct {
	URL string
}

type crawlResult struct {
	URL    string
	Page   *parser.HtmlPage
	Links  []string
	Cached bool
	Err    error
}

func NewService(repo domain.PageRepository) *Service {
	return &Service{
		client: http.Client{
			Timeout: 5 * time.Second,
		},
		repo: repo,
	}
}

/*
pre: existing URL string

post: byte array of HTML + error
*/
func (s *Service) Fetch(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MyCrawler/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"request to %s returned status %s",
			rawURL,
			resp.Status,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func resolveLink(currentPageURL string, href string) string {
	base, err := url.Parse(currentPageURL)
	if err != nil {
		return ""
	}

	link, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(link)

	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	resolved.Fragment = ""

	return resolved.String()
}

func (s *Service) saveKeywords(pageID int, page *parser.HtmlPage) error {
	keywordCounts := make(map[string]int)
	for _, keyword := range page.GetKeywords() {
		if keyword == "" {
			continue
		}

		keywordCounts[keyword]++
	}
	for keyword, count := range keywordCounts {
		if err := s.repo.SaveKeyword(
			pageID,
			keyword,
			count,
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) saveLinks(pageID int, page *parser.HtmlPage, currentURL string, startURL string) ([]string, error) {
	links := []string{}
	for _, rawLink := range page.GetLinks() {
		resolvedLink := resolveLink(currentURL, rawLink)
		if resolvedLink == "" {
			continue
		}
		toID, err := s.repo.SaveDiscoveredPage(resolvedLink)
		if err != nil {
			return nil, err
		}
		if err := s.repo.SaveLink(
			pageID,
			toID,
			resolvedLink,
		); err != nil {
			return nil, err
		}
		if !sameHost(startURL, resolvedLink) {
			continue
		}
		links = append(links, resolvedLink)
	}

	return links, nil
}

/*
pre: URL string + crawl limit

post: array of crawled HtmlPage + error
*/
func (s *Service) Crawl(startURL string, maxPages int) ([]*parser.HtmlPage, error) {
	return s.CrawlWithProgress(startURL, maxPages, nil)
}

func (s *Service) CrawlWithProgress(
	startURL string,
	maxPages int,
	onProgress func(Progress),
) ([]*parser.HtmlPage, error) {
	return s.CrawlWithWorkers(startURL, maxPages, 1, onProgress)
}

func (s *Service) CrawlWithWorkers(
	startURL string,
	maxPages int,
	workers int,
	onProgress func(Progress),
) ([]*parser.HtmlPage, error) {
	if maxPages <= 0 {
		return nil, nil
	}
	if workers <= 0 {
		workers = 1
	}
	if workers > maxPages {
		workers = maxPages
	}

	jobs := make(chan crawlJob)
	results := make(chan crawlResult)
	for i := 0; i < workers; i++ {
		go func() {
			for job := range jobs {
				results <- s.crawlOne(startURL, job.URL)
			}
		}()
	}

	queue := []string{startURL}
	visited := make(map[string]bool)
	queued := map[string]bool{
		startURL: true,
	}
	pages := make([]*parser.HtmlPage, 0, maxPages)
	inFlight := 0
	scheduled := 0
	var firstErr error

	sendJobs := func() {
		if firstErr != nil {
			return
		}
		for len(queue) > 0 && inFlight < workers && scheduled < maxPages {
			currentURL := queue[0]
			queue = queue[1:]
			if visited[currentURL] {
				continue
			}
			visited[currentURL] = true
			inFlight++
			scheduled++
			jobs <- crawlJob{URL: currentURL}
		}
	}

	sendJobs()
	for inFlight > 0 {
		result := <-results
		inFlight--

		if result.Err != nil {
			if firstErr == nil {
				firstErr = result.Err
				queue = nil
			}
			continue
		}
		if result.Page != nil {
			pages = append(pages, result.Page)
			for _, link := range result.Links {
				if queued[link] || visited[link] {
					continue
				}
				if scheduled+len(queue) >= maxPages {
					continue
				}
				queued[link] = true
				queue = append(queue, link)
			}
			if onProgress != nil {
				onProgress(Progress{
					Current: len(pages),
					Total:   maxPages,
					URL:     result.URL,
					Cached:  result.Cached,
				})
			}
		}

		sendJobs()
	}

	close(jobs)
	if firstErr != nil {
		return nil, firstErr
	}
	return pages, nil
}

func (s *Service) crawlOne(startURL string, currentURL string) crawlResult {
	storedPageID, storedHTML, alreadyCrawled, err := s.repo.GetPage(currentURL)
	if err != nil {
		return crawlResult{URL: currentURL, Err: err}
	}

	var pageID int
	var page *parser.HtmlPage
	if alreadyCrawled {
		pageID = storedPageID
		page = parser.Parse(storedHTML)
	} else {
		html, err := s.Fetch(currentURL)
		if err != nil {
			return crawlResult{URL: currentURL}
		}
		page = parser.Parse(html)
		pageID, err = s.repo.SavePage(page.GetTitle(), currentURL, html)
		if err != nil {
			return crawlResult{URL: currentURL, Err: err}
		}
		if err := s.saveKeywords(pageID, page); err != nil {
			return crawlResult{URL: currentURL, Err: err}
		}
	}

	links, err := s.saveLinks(pageID, page, currentURL, startURL)
	if err != nil {
		return crawlResult{URL: currentURL, Err: err}
	}

	return crawlResult{
		URL:    currentURL,
		Page:   page,
		Links:  links,
		Cached: alreadyCrawled,
	}
}

func sameHost(startURL, resolvedLink string) bool {
	start, err := url.Parse(startURL)
	if err != nil {
		return false
	}
	resolved, err := url.Parse(resolvedLink)
	if err != nil {
		return false
	}
	return start.Hostname() == resolved.Hostname()
}
