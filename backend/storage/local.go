package storage

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store interface {
	SavePage(title string, url string, html []byte) (int, error)
	SaveDiscoveredPage(url string) (int, error)
	GetPage(url string) (int, []byte, bool, error)
	SearchPages(query string) ([]Page, error)
	GetPageByID(id int) (Page, error)
	SaveKeyword(pageID int, keyword string, count int) error
	SaveLink(fromID int, toID int, link string) error
	Close() error
}

type Page struct {
	ID    int
	Title string
	URL   string
	HTML  []byte
}

type LocalStore struct {
	mu   sync.Mutex
	path string
	data localData
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

func NewDefaultStore() (Store, error) {
	return NewLocalStore(defaultLocalStorePath())
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

func defaultLocalStorePath() string {
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

	return os.Rename(tmpPath, s.path)
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
		return page.ID, s.save()
	}

	s.data.Pages[index].Title = title
	s.data.Pages[index].HTML = string(html)
	s.data.Pages[index].CrawledAt = &now
	return s.data.Pages[index].ID, s.save()
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
	return page.ID, s.save()
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

func (s *LocalStore) SearchPages(query string) ([]Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query = strings.ToLower(query)
	pages := []Page{}
	for _, page := range s.data.Pages {
		if page.CrawledAt == nil {
			continue
		}
		if !s.pageMatchesQuery(page, query) {
			continue
		}
		pages = append(pages, Page{
			ID:    page.ID,
			Title: page.Title,
			URL:   page.URL,
		})
	}

	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Title == pages[j].Title {
			return pages[i].ID < pages[j].ID
		}
		return pages[i].Title < pages[j].Title
	})
	return pages, nil
}

func (s *LocalStore) GetPageByID(id int) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, page := range s.data.Pages {
		if page.ID == id && page.CrawledAt != nil {
			return Page{
				ID:    page.ID,
				Title: page.Title,
				URL:   page.URL,
				HTML:  []byte(page.HTML),
			}, nil
		}
	}
	return Page{}, sql.ErrNoRows
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
	return s.save()
}

func (s *LocalStore) SaveLink(fromID int, toID int, link string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, existing := range s.data.Links {
		if existing.FromID == fromID && existing.ToID == toID {
			s.data.Links[index].Link = link
			return s.save()
		}
	}

	s.data.Links = append(s.data.Links, localLink{
		FromID: fromID,
		ToID:   toID,
		Link:   link,
	})
	return s.save()
}

func (s *LocalStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *LocalStore) pageMatchesQuery(page localPage, query string) bool {
	if strings.Contains(strings.ToLower(page.Title), query) {
		return true
	}
	if strings.Contains(strings.ToLower(page.URL), query) {
		return true
	}
	for keyword := range s.data.Keywords[page.ID] {
		if strings.Contains(strings.ToLower(keyword), query) {
			return true
		}
	}
	return false
}
