package domain

type PageRepository interface {
	SavePage(title string, url string, html []byte) (int, error)
	SaveDiscoveredPage(url string) (int, error)
	GetPage(url string) (int, []byte, bool, error)
	SearchPages(query string) ([]Page, error)
	GetPageByID(id int) (Page, error)
	SaveKeyword(pageID int, keyword string, count int) error
	SaveLink(fromID int, toID int, link string) error
	Close() error
}
