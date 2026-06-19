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
	time.Sleep(500 * time.Millisecond)
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

func (s *Service) enqueueLinks(
	pageID int,
	page *parser.HtmlPage,
	currentURL string,
	startURL string,
	queue *[]string,
	visited map[string]bool,
	queued map[string]bool,
) error {
	for _, rawLink := range page.GetLinks() {
		resolvedLink := resolveLink(currentURL, rawLink)
		if resolvedLink == "" {
			continue
		}
		toID, err := s.repo.SaveDiscoveredPage(resolvedLink)
		if err != nil {
			return err
		}
		if err := s.repo.SaveLink(
			pageID,
			toID,
			resolvedLink,
		); err != nil {
			return err
		}
		if !sameHost(startURL, resolvedLink) {
			continue
		}
		if visited[resolvedLink] || queued[resolvedLink] {
			continue
		}
		*queue = append(*queue, resolvedLink)
		queued[resolvedLink] = true
	}

	return nil
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
	queue := []string{startURL}

	visited := make(map[string]bool)
	queued := map[string]bool{
		startURL: true,
	}
	pages := make([]*parser.HtmlPage, 0, maxPages)

	for len(queue) > 0 && len(pages) < maxPages {
		currentURL := queue[0]
		queue = queue[1:]
		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true
		storedPageID, storedHTML, alreadyCrawled, err := s.repo.GetPage(currentURL)
		if err != nil {
			return nil, err
		}

		var pageID int
		var page *parser.HtmlPage
		cached := alreadyCrawled

		if alreadyCrawled {
			pageID = storedPageID
			page = parser.Parse(storedHTML)
		} else {
			html, err := s.Fetch(currentURL)
			if err != nil {
				fmt.Printf("failed to fetch %s: %v\n", currentURL, err)
				continue
			}
			page = parser.Parse(html)
			pageID, err = s.repo.SavePage(
				page.GetTitle(),
				currentURL,
				html,
			)
			if err != nil {
				return nil, err
			}
			if err := s.saveKeywords(pageID, page); err != nil {
				return nil, err
			}
		}

		pages = append(pages, page)
		if err := s.enqueueLinks(
			pageID,
			page,
			currentURL,
			startURL,
			&queue,
			visited,
			queued,
		); err != nil {
			return nil, err
		}
		if onProgress != nil {
			onProgress(Progress{
				Current: len(pages),
				Total:   maxPages,
				URL:     currentURL,
				Cached:  cached,
			})
		}
	}
	return pages, nil
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
