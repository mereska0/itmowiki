package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mereska0/itmowiki/internal/domain"
)

type LocalStore struct {
	mu    sync.Mutex
	path  string
	data  localData
	dirty bool
}

type localData struct {
	NextID   int                    `json:"next_id"`
	Pages    []localPage            `json:"pages"`
	Keywords map[int]map[string]int `json:"keywords"`
	Links    []localLink            `json:"links"`
}

type localPage struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	HTML      string     `json:"html,omitempty"`
	CrawledAt *time.Time `json:"crawled_at,omitempty"`
}

type localLink struct {
	FromID int    `json:"from_id"`
	ToID   int    `json:"to_id"`
	Link   string `json:"link"`
}

func NewLocalStore(path string) (*LocalStore, error) {
	store := &LocalStore{
		path: path,
		data: localData{
			NextID:   1,
			Pages:    []localPage{},
			Keywords: map[int]map[string]int{},
			Links:    []localLink{},
		},
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func DefaultLocalStorePath() string {
	if path := os.Getenv("ITMOWIKI_DB_PATH"); path != "" {
		return path
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "itmowiki.json"
		}
		configDir = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configDir, "itmowiki", "data.json")
}

func (s *LocalStore) load() error {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&s.data); err != nil {
		return err
	}
	if s.data.NextID <= 0 {
		s.data.NextID = 1
	}
	if s.data.Keywords == nil {
		s.data.Keywords = map[int]map[string]int{}
	}
	if s.data.Links == nil {
		s.data.Links = []localLink{}
	}
	return nil
}

func (s *LocalStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func (s *LocalStore) SavePage(title string, url string, html []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	index := s.pageIndexByURL(url)
	if index == -1 {
		page := localPage{
			ID:        s.nextID(),
			Title:     title,
			URL:       url,
			HTML:      string(html),
			CrawledAt: &now,
		}
		s.data.Pages = append(s.data.Pages, page)
		s.dirty = true
		return page.ID, nil
	}

	s.data.Pages[index].Title = title
	s.data.Pages[index].HTML = string(html)
	s.data.Pages[index].CrawledAt = &now
	s.dirty = true
	return s.data.Pages[index].ID, nil
}

func (s *LocalStore) SaveDiscoveredPage(url string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.pageIndexByURL(url)
	if index != -1 {
		return s.data.Pages[index].ID, nil
	}

	page := localPage{
		ID:  s.nextID(),
		URL: url,
	}
	s.data.Pages = append(s.data.Pages, page)
	s.dirty = true
	return page.ID, nil
}

func (s *LocalStore) GetPage(url string) (int, []byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.pageIndexByURL(url)
	if index == -1 {
		return 0, nil, false, nil
	}

	page := s.data.Pages[index]
	return page.ID, []byte(page.HTML), page.CrawledAt != nil, nil
}

func (s *LocalStore) SearchPages(query string) ([]domain.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query = strings.ToLower(query)
	incomingLinks := s.incomingLinkCounts()
	pages := []domain.Page{}
	for _, page := range s.data.Pages {
		if page.CrawledAt == nil {
			continue
		}
		score := s.pageScore(page, query, incomingLinks[page.ID])
		if score == 0 {
			continue
		}
		pages = append(pages, domain.Page{
			ID:    page.ID,
			Title: page.Title,
			URL:   page.URL,
			Score: score,
		})
	}

	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Score != pages[j].Score {
			return pages[i].Score > pages[j].Score
		}
		if pages[i].Title == pages[j].Title {
			return pages[i].ID < pages[j].ID
		}
		return pages[i].Title < pages[j].Title
	})
	return pages, nil
}

func (s *LocalStore) Stats() (domain.IndexStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := domain.IndexStats{
		PagesDiscovered: len(s.data.Pages),
		LinksStored:     len(s.data.Links),
	}
	for _, page := range s.data.Pages {
		if page.CrawledAt != nil {
			stats.PagesCrawled++
		}
	}

	keywordTotals := make(map[string]int)
	for _, keywords := range s.data.Keywords {
		for keyword, count := range keywords {
			keywordTotals[keyword] += count
		}
	}
	stats.KeywordsIndexed = len(keywordTotals)
	stats.TopKeywords = topKeywords(keywordTotals, 10)
	stats.TopLinkedPages = s.topLinkedPages(10)

	return stats, nil
}

func (s *LocalStore) GetPageByID(id int) (domain.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, page := range s.data.Pages {
		if page.ID == id && page.CrawledAt != nil {
			return domain.Page{
				ID:    page.ID,
				Title: page.Title,
				URL:   page.URL,
				HTML:  []byte(page.HTML),
			}, nil
		}
	}
	return domain.Page{}, domain.ErrPageNotFound
}

func (s *LocalStore) SaveKeyword(pageID int, keyword string, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.Keywords == nil {
		s.data.Keywords = map[int]map[string]int{}
	}
	if s.data.Keywords[pageID] == nil {
		s.data.Keywords[pageID] = map[string]int{}
	}
	s.data.Keywords[pageID][keyword] = count
	s.dirty = true
	return nil
}

func (s *LocalStore) SaveLink(fromID int, toID int, link string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, existing := range s.data.Links {
		if existing.FromID == fromID && existing.ToID == toID {
			s.data.Links[index].Link = link
			s.dirty = true
			return nil
		}
	}

	s.data.Links = append(s.data.Links, localLink{
		FromID: fromID,
		ToID:   toID,
		Link:   link,
	})
	s.dirty = true
	return nil
}

func (s *LocalStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}
	return s.save()
}

func (s *LocalStore) pageIndexByURL(url string) int {
	for index, page := range s.data.Pages {
		if page.URL == url {
			return index
		}
	}
	return -1
}

func (s *LocalStore) nextID() int {
	id := s.data.NextID
	s.data.NextID++
	return id
}

func (s *LocalStore) pageScore(page localPage, query string, incomingLinks int) int {
	score := 0
	if strings.Contains(strings.ToLower(page.Title), query) {
		score += 10
	}
	if strings.Contains(strings.ToLower(page.URL), query) {
		score++
	}
	for keyword, count := range s.data.Keywords[page.ID] {
		if strings.Contains(strings.ToLower(keyword), query) {
			score += count * 2
		}
	}
	if score > 0 {
		score += incomingLinks
	}
	return score
}

func (s *LocalStore) incomingLinkCounts() map[int]int {
	counts := make(map[int]int)
	for _, link := range s.data.Links {
		counts[link.ToID]++
	}
	return counts
}

func (s *LocalStore) topLinkedPages(limit int) []domain.LinkedPageStat {
	incomingLinks := s.incomingLinkCounts()
	pagesByID := make(map[int]localPage, len(s.data.Pages))
	for _, page := range s.data.Pages {
		pagesByID[page.ID] = page
	}

	top := make([]domain.LinkedPageStat, 0, len(incomingLinks))
	for pageID, count := range incomingLinks {
		page := pagesByID[pageID]
		top = append(top, domain.LinkedPageStat{
			PageID:        pageID,
			Title:         page.Title,
			URL:           page.URL,
			IncomingLinks: count,
		})
	}

	sort.Slice(top, func(i, j int) bool {
		if top[i].IncomingLinks != top[j].IncomingLinks {
			return top[i].IncomingLinks > top[j].IncomingLinks
		}
		if top[i].Title == top[j].Title {
			return top[i].PageID < top[j].PageID
		}
		return top[i].Title < top[j].Title
	})
	if len(top) > limit {
		return top[:limit]
	}
	return top
}

func topKeywords(keywordTotals map[string]int, limit int) []domain.KeywordStat {
	top := make([]domain.KeywordStat, 0, len(keywordTotals))
	for keyword, count := range keywordTotals {
		top = append(top, domain.KeywordStat{
			Keyword: keyword,
			Count:   count,
		})
	}

	sort.Slice(top, func(i, j int) bool {
		if top[i].Count != top[j].Count {
			return top[i].Count > top[j].Count
		}
		return top[i].Keyword < top[j].Keyword
	})
	if len(top) > limit {
		return top[:limit]
	}
	return top
}
