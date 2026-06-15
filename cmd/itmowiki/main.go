package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	service "github.com/mereska0/itmowiki/backend/crawler"
	"github.com/mereska0/itmowiki/backend/parser/markdown"
	"github.com/mereska0/itmowiki/backend/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

const (
	defaultCrawlURL   = "https://neerc.ifmo.ru/wiki/index.php"
	defaultCrawlLimit = 100
)

type crawlProgressMsg service.Progress

type crawlDoneMsg struct {
	count int
	err   error
}

type tickMsg time.Time

type crawlModel struct {
	crawler  *service.Service
	startURL string
	maxPages int
	msgs     chan tea.Msg

	err     error
	count   int
	current int
	total   int
	url     string
	cached  bool
	frame   int
	done    bool
}

func (m crawlModel) Init() tea.Cmd {
	return tea.Batch(m.crawlCmd(), m.waitMsgCmd(), tickCmd())
}

func (m crawlModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case tickMsg:
		if m.done {
			return m, nil
		}
		m.frame++
		return m, tickCmd()
	case crawlProgressMsg:
		m.current = msg.Current
		m.total = msg.Total
		m.url = msg.URL
		m.cached = msg.Cached
		return m, m.waitMsgCmd()
	case crawlDoneMsg:
		m.count = msg.count
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m crawlModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("Ошибка загрузки: %v\n", m.err)
		}
		return fmt.Sprintf("Обработано страниц: %d\n", m.count)
	}

	total := m.total
	if total == 0 {
		total = m.maxPages
	}
	percent := 0
	if total > 0 {
		percent = m.current * 100 / total
	}
	if percent > 100 {
		percent = 100
	}

	status := "fetch"
	if m.cached {
		status = "cache"
	}
	spinner := []string{"|", "/", "-", "\\"}

	return fmt.Sprintf(
		"%s itmowiki crawl: %d%% (%d/%d) [%s]\n%s\n\nEsc/Ctrl+C - отменить\n",
		spinner[m.frame%len(spinner)],
		percent,
		m.current,
		total,
		status,
		shortText(m.url, 100),
	)
}

func (m crawlModel) crawlCmd() tea.Cmd {
	return func() tea.Msg {
		go func() {
			pages, err := m.crawler.CrawlWithProgress(
				m.startURL,
				m.maxPages,
				func(progress service.Progress) {
					m.msgs <- crawlProgressMsg(progress)
				},
			)
			m.msgs <- crawlDoneMsg{count: len(pages), err: err}
		}()
		return nil
	}
}

func (m crawlModel) waitMsgCmd() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgs
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func main() {
	if len(os.Args) == 1 {
		printHome()
		return
	}

	command := os.Args[1]
	if command == "help" || command == "--help" || command == "-h" {
		printHelp()
		return
	}

	store, err := storage.NewDefaultStore()
	if err != nil {
		log.Fatal("failed to open local storage:", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Println("failed to close storage:", err)
		}
	}()

	switch command {
	case "search":
		runSearch(store, os.Args[2:])
	case "crawl":
		runCrawl(store, os.Args[2:])
	case "show":
		runShow(store, os.Args[2:])
	default:
		fmt.Printf("Неизвестная команда: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHome() {
	fmt.Print(`ITMO Wiki

Команды:
  itmowiki search <запрос>
  itmowiki show <id>
  itmowiki crawl
  itmowiki crawl <url> [limit]
  itmowiki help
`)
}

func printHelp() {
	fmt.Print(`itmowiki - поиск по Викиконспектам

Использование:
  itmowiki
  itmowiki search <запрос>
  itmowiki show <id>
  itmowiki crawl
  itmowiki crawl <url> [limit]

Примеры:
  itmowiki search алгоритмы
  itmowiki search теория вероятностей
  itmowiki show 15
  itmowiki crawl
  itmowiki crawl "https://neerc.ifmo.ru/wiki/index.php?title=Заглавная_страница" 20
`)
}

func runSearch(store storage.Store, args []string) {
	if len(args) == 0 {
		log.Fatal("Использование: itmowiki search <запрос>")
	}

	query := strings.Join(args, " ")
	results, err := store.SearchPages(query)
	if err != nil {
		log.Fatal("search failed:", err)
	}

	if len(results) == 0 {
		fmt.Println("Ничего не найдено")
		return
	}

	fmt.Printf("Найдено страниц: %d\n\n", len(results))
	for _, result := range results {
		title := result.Title
		if title == "" {
			title = "(без заголовка)"
		}
		fmt.Printf("[%d] %s\n", result.ID, title)
		fmt.Println(result.URL)
		fmt.Println()
	}
}

func runShow(store storage.Store, args []string) {
	if len(args) != 1 {
		log.Fatal("Использование: itmowiki show <id>")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		log.Fatal("id должен быть числом")
	}

	page, err := store.GetPageByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		log.Fatalf("страница с id %d не найдена", id)
	}
	if err != nil {
		log.Fatal("failed to load page:", err)
	}

	if page.Title != "" {
		fmt.Println(page.Title)
	}
	fmt.Println(page.URL)
	fmt.Println()

	rendered, err := glamour.Render(markdown.Parse(page.HTML), "dark")
	if err != nil {
		log.Fatal("failed to render page:", err)
	}
	fmt.Println(rendered)
}

func runCrawl(store storage.Store, args []string) {
	startURL := defaultCrawlURL
	limit := defaultCrawlLimit
	if len(args) >= 1 {
		startURL = args[0]
	}
	if len(args) >= 2 {
		value, err := strconv.Atoi(args[1])
		if err != nil || value <= 0 {
			log.Fatal("limit должен быть положительным числом")
		}
		limit = value
	}
	if len(args) > 2 {
		log.Fatal("Использование: itmowiki crawl [url] [limit]")
	}

	crawlerService := service.NewService(store)
	if !isTerminal(os.Stdout) {
		pages, err := crawlerService.Crawl(startURL, limit)
		if err != nil {
			log.Fatal("crawl failed:", err)
		}
		fmt.Printf("Обработано страниц: %d\n", len(pages))
		return
	}

	model := crawlModel{
		crawler:  crawlerService,
		startURL: startURL,
		maxPages: limit,
		msgs:     make(chan tea.Msg, limit+2),
		total:    limit,
		url:      startURL,
	}
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		log.Fatal("loading UI failed:", err)
	}

	loadedModel := finalModel.(crawlModel)
	if loadedModel.err != nil {
		log.Fatal("crawl failed:", loadedModel.err)
	}
}

func shortText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	return string(runes[:maxLen-3]) + "..."
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
