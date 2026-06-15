package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

type Page struct {
	ID    int
	Title string
	URL   string
	HTML  []byte
}

func NewPostgresDB() *Postgres {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSL_MODE"),
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	// Create table if not exists
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS pages (
		id SERIAL PRIMARY KEY,
		title TEXT,
		url TEXT NOT NULL UNIQUE,
		html TEXT,
		crawled_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS keywords (
		id SERIAL PRIMARY KEY,
		word TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS page_to_keyword (
		page_id INT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
		keyword_id INT NOT NULL REFERENCES keywords(id) ON DELETE CASCADE,
		count INT NOT NULL DEFAULT 1,
		PRIMARY KEY (page_id, keyword_id)
	);

	CREATE TABLE IF NOT EXISTS links (
		from_id INT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
		to_id INT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
		link TEXT,
		UNIQUE (from_id, to_id)
	);
`)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	return &Postgres{db}
}

func (p *Postgres) SavePage(title string, url string, html []byte) (int, error) {
	var pageID int

	err := p.db.QueryRow(`
		INSERT INTO pages (title, url, html, crawled_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (url)
		DO UPDATE SET
			title = EXCLUDED.title,
			html = EXCLUDED.html,
			crawled_at = NOW()
		RETURNING id
	`,
		title,
		url,
		string(html),
	).Scan(&pageID)

	return pageID, err
}

func (p *Postgres) SaveKeyword(pageID int, keyword string, count int) error {
	var keywordID int

	err := p.db.QueryRow(`
		INSERT INTO keywords (word)
		VALUES ($1)
		ON CONFLICT (word)
		DO UPDATE SET word = EXCLUDED.word
		RETURNING id
	`, keyword).Scan(&keywordID)

	if err != nil {
		return err
	}

	_, err = p.db.Exec(`
		INSERT INTO page_to_keyword (page_id, keyword_id, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (page_id, keyword_id)
		DO UPDATE SET count = EXCLUDED.count
	`, pageID, keywordID, count)

	return err
}

func (p *Postgres) SaveLink(fromID int, toID int, link string) error {
	_, err := p.db.Exec(`
		INSERT INTO links (from_id, to_id, link)
		VALUES ($1, $2, $3)
		ON CONFLICT (from_id, to_id)
		DO UPDATE SET link = EXCLUDED.link
	`, fromID, toID, link)

	return err
}

func (p *Postgres) SaveDiscoveredPage(url string) (int, error) {
	var pageID int

	err := p.db.QueryRow(`
		INSERT INTO pages (url)
		VALUES ($1)
		ON CONFLICT (url)
		DO UPDATE SET url = EXCLUDED.url
		RETURNING id
	`, url).Scan(&pageID)

	return pageID, err
}

func (p *Postgres) GetPage(url string) (int, []byte, bool, error) {
	var pageID int
	var html sql.NullString
	var crawled bool

	err := p.db.QueryRow(`
		SELECT id, html, crawled_at IS NOT NULL
		FROM pages
		WHERE url = $1
	`, url).Scan(&pageID, &html, &crawled)

	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}

	if err != nil {
		return 0, nil, false, err
	}

	return pageID, []byte(html.String), crawled, nil
}

func (p *Postgres) SearchPages(query string) ([]Page, error) {
	rows, err := p.db.Query(`
		SELECT DISTINCT p.id, COALESCE(p.title, ''), p.url
		FROM pages p
		LEFT JOIN page_to_keyword ptk ON ptk.page_id = p.id
		LEFT JOIN keywords k ON k.id = ptk.keyword_id
		WHERE p.crawled_at IS NOT NULL
			AND (
				LOWER(COALESCE(p.title, '')) LIKE '%' || LOWER($1) || '%'
				OR LOWER(p.url) LIKE '%' || LOWER($1) || '%'
				OR LOWER(COALESCE(k.word, '')) LIKE '%' || LOWER($1) || '%'
			)
		ORDER BY COALESCE(p.title, ''), p.id
	`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := []Page{}
	for rows.Next() {
		var page Page
		if err := rows.Scan(&page.ID, &page.Title, &page.URL); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pages, nil
}

func (p *Postgres) GetPageByID(id int) (Page, error) {
	var page Page
	var html sql.NullString

	err := p.db.QueryRow(`
		SELECT id, COALESCE(title, ''), url, html
		FROM pages
		WHERE id = $1 AND crawled_at IS NOT NULL
	`, id).Scan(&page.ID, &page.Title, &page.URL, &html)
	if err != nil {
		return Page{}, err
	}

	page.HTML = []byte(html.String)
	return page, nil
}

func (p *Postgres) Close() error {
	return p.db.Close()
}
